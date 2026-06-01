package definitions

type Definition struct {
	Id               string `yaml:"id"`
	Name             string `yaml:"name"`
	ShortDescription string `yaml:"short_description"`
	LongDescription  string `yaml:"long_description"`
	Category         string `yaml:"category"`
}
