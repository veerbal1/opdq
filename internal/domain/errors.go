package domain

import "errors"

var ErrSessionNotFound = errors.New("session not found")
var ErrAppointmentNotFound = errors.New("appointment not found")
var ErrIllegalTransition = errors.New("illegal transition")
var ErrSessionEnded = errors.New("session has ended")
var ErrInvalidSessionTimes = errors.New("ends_at must be after starts_at")
var ErrInvalidCapacity = errors.New("capacity must be greater than zero")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUnauthorized = errors.New("unauthorized")
