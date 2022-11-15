package analysis

type ViewFactory struct {
	Name        string
	Description string
	Create      ViewFactoryFunction
}
type ViewFactoryFunction func(results *Results) *View

type View struct {
	Name    string
	Columns []*Column
	Rows    []*Row
}
type RowData map[string]interface{}

type Row struct {
	Data RowData
}

const (
	Integer = iota
	Float
	String
	Date
)

type Column struct {
	Name string
	Type int
}

func StringColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: String,
	}
}
func IntColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Integer,
	}
}

func FloatColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Float,
	}
}
func DateColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Date,
	}
}
