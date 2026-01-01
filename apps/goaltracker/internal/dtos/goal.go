package dtos

type LinkGoalDto struct {
	TypeID      int64   `json:"typeId"`
	TargetValue *int64  `json:"targetValue"`
	Tag         *string `json:"tag"`
}

func (dto *LinkGoalDto) Validate() (bool, map[string]string) {
	return true, make(map[string]string)
}
