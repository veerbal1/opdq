package domain

import "errors"

var ErrSessionNotFound = errors.New("session not found")
var ErrAppointmentNotFound = errors.New("appointment not found")
var ErrIllegalTransition = errors.New("illegal transition")
