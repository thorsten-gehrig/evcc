package config

type Typed struct {
	Type  string
	Other map[string]interface{} `mapstructure:",remain"`
}

type Named struct {
	Name, Type string
	Other      map[string]interface{} `mapstructure:",remain"`
}
