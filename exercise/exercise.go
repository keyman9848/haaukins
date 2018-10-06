package exercise

import (
	"errors"
	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
)

var (
	DuplicateTagErr = errors.New("Tag already exists")
	MissingTagsErr  = errors.New("No tags, need atleast one tag")
	UnknownTagErr   = errors.New("Unknown tag")
)

type Flag struct {
}

type RecordConfig struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`
	RData string `yaml:"rdata"`
}

type FlagConfig struct {
	Name    string `yaml:"name"`
	EnvVar  string `yaml:"env"`
	Default string `yaml:"default"`
	Points  uint   `yaml:"points"`
}

type EnvVarConfig struct {
	EnvVar string `yaml:"env"`
	Value  string `yaml:"value"`
}

type DockerConfig struct {
	Image    string         `yaml:"image"`
	Flags    []FlagConfig   `yaml:"flag"`
	Envs     []EnvVarConfig `yaml:"env"`
	Records  []RecordConfig `yaml:"dns"`
	MemoryMB uint           `yaml:"memoryMB"`
	CPU      float64        `yaml:"cpu"`
}

type VBoxConfig struct {
	Image    string       `yaml:"image"`
	MemoryMB uint         `yaml:"memoryMB"`
	Flags    []FlagConfig `yaml:"flag"`
}

type Config struct {
	Name        string         `yaml:"name"`
	Tags        []string       `yaml:"tags"`
	DockerConfs []DockerConfig `yaml:"docker"`
	VBoxConfig  []VBoxConfig   `yaml:"vbox"`
}

func (conf Config) Flags() []FlagConfig {
	var res []FlagConfig
	for _, dockerConf := range conf.DockerConfs {
		res = append(res, dockerConf.Flags...)
	}
	for _, vboxConf := range conf.VBoxConfig {
		res = append(res, vboxConf.Flags...)
	}
	return res
}

func (ec Config) ContainerOpts() ([]docker.ContainerConfig, [][]RecordConfig) {
	var contSpecs []docker.ContainerConfig
	var contRecords [][]RecordConfig

	for _, conf := range ec.DockerConfs {
		envVars := make(map[string]string)

		for _, flag := range conf.Flags {
			envVars[flag.EnvVar] = flag.Default
		}

		for _, env := range conf.Envs {
			envVars[env.EnvVar] = env.Value
		}

		// docker config
		spec := docker.ContainerConfig{
			Image: conf.Image,
			Resources: &docker.Resources{
				MemoryMB: conf.MemoryMB,
				CPU:      conf.CPU,
			},
			EnvVars: envVars,
		}

		contSpecs = append(contSpecs, spec)
		contRecords = append(contRecords, conf.Records)
	}

	return contSpecs, contRecords
}

type DockerHost interface {
	CreateContainer(conf docker.ContainerConfig) (docker.Container, error)
}

type dockerHost struct{}

func (dockerHost) CreateContainer(conf docker.ContainerConfig) (docker.Container, error) {
	return docker.NewContainer(conf)
}

type exercise struct {
	conf       *Config
	net        docker.Network
	flags      []Flag
	machines   []virtual.Instance
	ips        []int
	dnsIP      string
	dnsRecords []RecordConfig
	dockerHost DockerHost
	lib        vbox.Library
}

func (e *exercise) Create() error {
	containers, records := e.conf.ContainerOpts()

	var machines []virtual.Instance
	var newIps []int
	for i, spec := range containers {
		spec.DNS = []string{e.dnsIP}

		c, err := e.dockerHost.CreateContainer(spec)
		if err != nil {
			return err
		}

		var lastDigit int
		// Example: 216

		if e.ips != nil {
			// Containers need specific ips
			lastDigit, err = e.net.Connect(c, spec.MacAddress, e.ips[i])
			if err != nil {
				return err
			}
		} else {
			// Let network assign ips
			lastDigit, err = e.net.Connect(c, spec.MacAddress)
			if err != nil {
				return err
			}

			newIps = append(newIps, lastDigit)
		}

		ipaddr := e.net.FormatIP(lastDigit)
		// Example: 172.16.5.216

		for _, record := range records[i] {
			if record.RData == "" {
				record.RData = ipaddr
			}
			e.dnsRecords = append(e.dnsRecords, record)
		}

		machines = append(machines, c)
	}

	for _, spec := range e.conf.VBoxConfig {
		vm, err := e.lib.GetCopy(
			spec.Image,
			vbox.SetBridge(e.net.Interface()),
		)
		if err != nil {
			return err
		}
		machines = append(machines, vm)
	}

	if e.ips == nil {
		e.ips = newIps
	}

	e.machines = machines

	return nil
}

func (e *exercise) Start() error {
	for _, m := range e.machines {
		if err := m.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (e *exercise) Stop() error {
	for _, m := range e.machines {
		if err := m.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (e *exercise) Close() error {
	for _, m := range e.machines {
		if err := m.Close(); err != nil {
			return err
		}
	}
	e.machines = nil
	return nil
}

func (e *exercise) Reset() error {
	if err := e.Stop(); err != nil {
		return err
	}

	if err := e.Start(); err != nil {
		return err
	}

	return nil
}
