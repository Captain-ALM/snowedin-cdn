package structure

type ConfigYaml struct {
	Listen ListenYaml `yaml:"listen"`
}

type ListenYaml struct {
	Web string `yaml:"web"`
	Api string `yaml:"api"`
}
