package config

type PackageDescriptor struct {
	Image string `json:"image"`
}

type Configuration struct {
	AdditionalPackages []PackageDescriptor `json:"additionalPackages"`
}

var (
	DefaultConfig = Configuration{
		AdditionalPackages: []PackageDescriptor{},
	}
)
