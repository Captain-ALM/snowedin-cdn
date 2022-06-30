package conf

type CacheSettingsYaml struct {
	MaxAge                               uint `yaml:"maxAge"`
	PrivateCache                         bool `yaml:"privateCache"`
	NotModifiedResponseUsingLastModified bool `yaml:"notModifiedUsingLastModified"`
	NotModifiedResponseUsingETags        bool `yaml:"notModifiedUsingETags"`
	RequestLimitedCacheCheck             bool `yaml:"requestLimitedCacheCheck"`
}
