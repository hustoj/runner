package runner

const (
	OJ_WT0 = 0
	OJ_WT1 = 1
	OJ_CI = 2
	OJ_RI = 3
	OJ_AC = 4
	OJ_PE = 5
	OJ_WA = 6
	OJ_TL = 7
	OJ_ML = 8
	OJ_OL = 9
	OJ_RE = 10
	OJ_CE = 11
	OJ_CO = 12
)


type Result struct {
	RetCode int
	Memory int
	TimeCost int
}

func NewResult() Result{
	return Result{
		RetCode: OJ_AC,
		Memory: 0,
		TimeCost: 0,
	}
}

func (res *Result) Accept(memory int, duration int)  {
	res.Memory = memory
	res.TimeCost = duration
}

func (res *Result) Wrong(memory int, duration int){
	res.RetCode = OJ_WA
	res.Memory = memory
	res.TimeCost = duration
}

func (res *Result) Tle() {
	res.RetCode = OJ_TL
}