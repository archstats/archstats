package analysis

type View struct {
	Name    string
	Columns []*Column
	Rows    []*Row
}

type ViewFactory struct {
	Name           string
	CreateViewFunc ViewFactoryFunction
}
type ViewFactoryFunction func(results *Results) *View

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
	PositionInFile
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
func PositionInFileColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: PositionInFile,
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
