package domain

type State string

const Waiting State = "waiting"
const InConsultation State = "in_consultation"
const Done State = "done"
const Absent State = "absent"

var allowedTransitions = map[State]map[State]bool{
	Waiting: {
		InConsultation: true,
		Absent:         true,
	},
	InConsultation: {
		Done:    true,
		Waiting: true,
	},
	Absent: {
		Waiting: true,
	},
}

func Transition(from, to State) error {
	section := allowedTransitions[from]

	if section[to] {
		return nil
	}

	return ErrIllegalTransition
}
