package md0

type Document struct {
	Path             string
	Source           string
	LanguageVersion  string
	LanguageDeclared bool
	Nodes            []Node
}

type Node interface {
	LineNo() int
}

type MarkdownNode struct {
	Line int
	Text string
}

func (n MarkdownNode) LineNo() int { return n.Line }

type InputNode struct {
	Line                              int
	Prefix, Name, Type, DefaultSource string
	Default                           Expr
}

func (n InputNode) LineNo() int { return n.Line }

type CalcNode struct {
	Line         int
	Name, Source string
	Expr         Expr
}

func (n CalcNode) LineNo() int { return n.Line }

type DataNode struct {
	Line                 int
	Name, Format         string
	FileName, FileSHA256 string
	Value                Value
}

func (n DataNode) LineNo() int { return n.Line }

type ShowNode struct {
	Line   int
	Source string
	Expr   Expr
}

func (n ShowNode) LineNo() int { return n.Line }

type AssertNode struct {
	Line    int
	Source  string
	Expr    Expr
	Message string
}

func (n AssertNode) LineNo() int { return n.Line }

type WhenNode struct {
	Line   int
	Source string
	Expr   Expr
	Nodes  []Node
}

func (n WhenNode) LineNo() int { return n.Line }

type ChartNode struct {
	Line                       int
	Name, Type                 string
	LabelsSource, ValuesSource string
	Labels, Values             Expr
}

func (n ChartNode) LineNo() int { return n.Line }

type TableNode struct {
	Line                            int
	Name, ColumnsSource, RowsSource string
	Columns, Rows                   Expr
}

func (n TableNode) LineNo() int { return n.Line }
