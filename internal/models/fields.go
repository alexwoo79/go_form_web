package models

type FieldInfo struct {
	Name        string
	Label       string
	Type        string
	Placeholder string
	Required    bool
	Options     []string
	Min         *float64
	Max         *float64
	Step        *float64
}
