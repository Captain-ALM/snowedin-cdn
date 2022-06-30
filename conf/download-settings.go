package conf

type DownloadSettingsYaml struct {
	OutputDisposition     bool `yaml:"outputDisposition"`
	OutputFilename        bool `yaml:"outputFilename"`
	SetExtensionIfMissing bool `yaml:"setExtensionIfMissing"`
}
