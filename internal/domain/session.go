package domain

import "time"

func ValidateSession(startsAt, endsAt time.Time, capacity int) error {
	if !endsAt.After(startsAt) {
		return ErrInvalidSessionTimes
	}
	if capacity <= 0 {
		return ErrInvalidCapacity
	}
	return nil
}
