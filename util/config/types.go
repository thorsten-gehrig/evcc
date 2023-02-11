package config

type container[T any] struct {
	config Named
	device T
}

type Typed struct {
	Type  string
	Other map[string]interface{} `mapstructure:",remain"`
}

type Named struct {
	Name, Type string
	Other      map[string]interface{} `mapstructure:",remain"`
}
