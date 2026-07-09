package harness

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// ServiceSpec describes a reference sidecar in a compose challenge.
type ServiceSpec struct {
	Name               string // e.g. "idgen"
	ReferenceChallenge string
	ReferenceLang      string // default "go"
	EnvAddrKey         string // env var injected into gateway, e.g. IDGEN_ADDR
	ExtraArgs          []string
	// ClusterSize > 0 spawns a multi-node reference cluster (e.g. 3 Raft nodes).
	// Env vars become <EnvAddrKey>1_ADDR, <EnvAddrKey>2_ADDR, … (e.g. RAFT1_ADDR).
	ClusterSize int
}

// Compose manages a user gateway plus reference service subprocesses.
type Compose struct {
	gateway  *Program
	services []serviceProc
	logf     func(format string, args ...any)
}

type serviceProc struct {
	spec    ServiceSpec
	program *Program
	cluster *Cluster
}

// ComposeContext is passed to compose stage tests.
type ComposeContext struct {
	compose *Compose
	Logf    func(format string, args ...any)
}

// DialGateway opens a client to the user's gateway.
func (c *ComposeContext) DialGateway() (*Client, error) {
	return Dial(c.compose.gateway.Addr())
}

// DialService opens a client to a reference service by name.
func (c *ComposeContext) DialService(name string) (*Client, error) {
	addr, err := c.compose.ServiceAddr(name)
	if err != nil {
		return nil, err
	}
	return Dial(addr)
}

// ClusterNodeAddr returns the loopback address for a node in a reference cluster.
func (c *ComposeContext) ClusterNodeAddr(service string, nodeID int) (string, error) {
	return c.compose.ClusterNodeAddr(service, nodeID)
}

// DialClusterNode opens a client to a node in a reference cluster service.
func (c *ComposeContext) DialClusterNode(service string, nodeID int) (*Client, error) {
	addr, err := c.compose.ClusterNodeAddr(service, nodeID)
	if err != nil {
		return nil, err
	}
	return Dial(addr)
}

// ServiceAddr returns the loopback address for a reference service.
func (c *ComposeContext) ServiceAddr(name string) (string, error) {
	return c.compose.ServiceAddr(name)
}

// NewCompose starts reference services, then the user gateway with peer env.
func NewCompose(gatewayPath string, specs []ServiceSpec, logf func(string, ...any)) (*Compose, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("compose requires at least one ServiceSpec")
	}
	abs, err := filepath.Abs(gatewayPath)
	if err != nil {
		return nil, err
	}
	c := &Compose{logf: logf}
	env := map[string]string{}

	for _, spec := range specs {
		lang := spec.ReferenceLang
		if lang == "" {
			lang = "go"
		}
		refPath, err := ReferenceProgramPath(spec.ReferenceChallenge, lang)
		if err != nil {
			c.Cleanup()
			return nil, fmt.Errorf("service %q: %w", spec.Name, err)
		}
		sp := serviceProc{spec: spec}
		if spec.ClusterSize > 0 {
			cluster, err := NewCluster(refPath, spec.ClusterSize, logf)
			if err != nil {
				c.Cleanup()
				return nil, fmt.Errorf("service %q cluster: %w", spec.Name, err)
			}
			if err := cluster.StartAll(); err != nil {
				cluster.Cleanup()
				c.Cleanup()
				return nil, fmt.Errorf("starting reference cluster %q: %w", spec.Name, err)
			}
			sp.cluster = cluster
			if spec.EnvAddrKey != "" {
				for i := 1; i <= spec.ClusterSize; i++ {
					addr, err := cluster.Addr(i)
					if err != nil {
						cluster.Cleanup()
						c.Cleanup()
						return nil, err
					}
					env[spec.EnvAddrKey+strconv.Itoa(i)+"_ADDR"] = addr
				}
			}
			c.services = append(c.services, sp)
			continue
		}
		prog, err := NewProgram(refPath, logf)
		if err != nil {
			c.Cleanup()
			return nil, err
		}
		if err := prog.StartWithArgs(spec.ExtraArgs, nil); err != nil {
			prog.Cleanup()
			c.Cleanup()
			return nil, fmt.Errorf("starting reference %q: %w", spec.Name, err)
		}
		sp.program = prog
		c.services = append(c.services, sp)
		if spec.EnvAddrKey != "" {
			env[spec.EnvAddrKey] = prog.Addr()
		}
	}

	gw, err := NewProgram(abs, logf)
	if err != nil {
		c.Cleanup()
		return nil, err
	}
	if err := gw.StartWithArgs(nil, env); err != nil {
		gw.Cleanup()
		c.Cleanup()
		return nil, fmt.Errorf("starting gateway: %w", err)
	}
	c.gateway = gw

	logf("compose: gateway=%s", gw.Addr())
	for _, sp := range c.services {
		if sp.cluster != nil {
			for i := 1; i <= sp.cluster.Size(); i++ {
				addr, _ := sp.cluster.Addr(i)
				logf("  %s (%s) node%d → %s", sp.spec.Name, sp.spec.ReferenceChallenge, i, addr)
			}
			continue
		}
		logf("  %s (%s) → %s", sp.spec.Name, sp.spec.ReferenceChallenge, sp.program.Addr())
	}
	return c, nil
}

// ClusterNodeAddr returns the address of a node in a reference cluster service.
func (c *Compose) ClusterNodeAddr(name string, nodeID int) (string, error) {
	for _, sp := range c.services {
		if sp.spec.Name == name && sp.cluster != nil {
			return sp.cluster.Addr(nodeID)
		}
	}
	return "", fmt.Errorf("unknown compose cluster service %q", name)
}

// ServiceAddr returns the address of a named reference service.
func (c *Compose) ServiceAddr(name string) (string, error) {
	for _, sp := range c.services {
		if sp.spec.Name == name {
			if sp.cluster != nil {
				return sp.cluster.Addr(1)
			}
			return sp.program.Addr(), nil
		}
	}
	return "", fmt.Errorf("unknown compose service %q", name)
}

// Cleanup stops the gateway and all reference services.
func (c *Compose) Cleanup() {
	if c.gateway != nil {
		c.gateway.Cleanup()
		c.gateway = nil
	}
	for _, sp := range c.services {
		if sp.cluster != nil {
			sp.cluster.Cleanup()
		} else if sp.program != nil {
			sp.program.Cleanup()
		}
	}
	c.services = nil
}
