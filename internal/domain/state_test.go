package domain

import (
	"testing"
)

func TestTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    State
		to      State
		wantErr bool
	}{
		{
			name:    "waiting to waiting - illegal",
			from:    Waiting,
			to:      Waiting,
			wantErr: true,
		},
		{
			name:    "waiting to in_consultation - legal",
			from:    Waiting,
			to:      InConsultation,
			wantErr: false,
		},
		{
			name:    "waiting to done - illegal",
			from:    Waiting,
			to:      Done,
			wantErr: true,
		},
		{
			name:    "waiting to absent - legal",
			from:    Waiting,
			to:      Absent,
			wantErr: false,
		},
		{
			name:    "in_consultation to waiting - legal",
			from:    InConsultation,
			to:      Waiting,
			wantErr: false,
		},
		{
			name:    "in_consultation to in_consultation - illegal",
			from:    InConsultation,
			to:      InConsultation,
			wantErr: true,
		},
		{
			name:    "in_consultation to done - legal",
			from:    InConsultation,
			to:      Done,
			wantErr: false,
		},
		{
			// "Call" announces the token; the patient may simply not turn up.
			name:    "in_consultation to absent - legal",
			from:    InConsultation,
			to:      Absent,
			wantErr: false,
		},
		{
			name:    "done to waiting - illegal",
			from:    Done,
			to:      Waiting,
			wantErr: true,
		},
		{
			name:    "done to in_consultation - illegal",
			from:    Done,
			to:      InConsultation,
			wantErr: true,
		},
		{
			name:    "done to done - illegal",
			from:    Done,
			to:      Done,
			wantErr: true,
		},
		{
			name:    "done to absent - illegal",
			from:    Done,
			to:      Absent,
			wantErr: true,
		},
		{
			name:    "absent to waiting - legal",
			from:    Absent,
			to:      Waiting,
			wantErr: false,
		},
		{
			name:    "absent to in_consultation - illegal",
			from:    Absent,
			to:      InConsultation,
			wantErr: true,
		},
		{
			name:    "absent to done - illegal",
			from:    Absent,
			to:      Done,
			wantErr: true,
		},
		{
			name:    "absent to absent - illegal",
			from:    Absent,
			to:      Absent,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Transition(tt.from, tt.to)
			gotErr := err != nil

			if gotErr != tt.wantErr {
				t.Errorf("Transition(%v, %v) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}
