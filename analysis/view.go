package analysis

type ViewFactory struct {
	Name           string
	Description    string
	CreateViewFunc ViewFactoryFunction
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
type ColumnType int

const (
	Integer ColumnType = iota
	Float
	String
	Date
)

type Column struct {
	Name string
	Type ColumnType
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
