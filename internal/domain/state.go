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
		// "Call" means the token was announced, not that the patient walked in.
		// Announcing and nobody coming is the ordinary way a no-show happens, so
		// the desk must be able to mark it from here — without first bouncing the
		// patient back to waiting, which would reset queued_at and move them in
		// line for no reason.
		Absent: true,
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
