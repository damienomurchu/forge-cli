package domain

import "time"

// WithStatus returns a record with the requested lifecycle status. A no-op
// preserves the record exactly; a real change updates only status and UpdatedAt.
func (r Record) WithStatus(status Status, now time.Time) (Record, error) {
	if err := r.Validate(); err != nil {
		return Record{}, err
	}
	if !status.Valid() {
		return Record{}, &InvalidValueError{Field: "status", Value: status.String()}
	}
	if status == r.Status {
		return r, nil
	}

	updatedAt := NewTimestamp(now)
	if updatedAt.Time().Before(r.CreatedAt.Time()) {
		return Record{}, &InvalidValueError{Field: "updated_at", Value: updatedAt.String()}
	}
	r.Status = status
	r.UpdatedAt = updatedAt
	return r, nil
}
