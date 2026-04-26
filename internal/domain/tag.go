package domain

// WorkTag represents a tag applied to a work (shared/global).
type WorkTag struct {
	WorkID int64  `db:"work_id" json:"work_id"`
	Tag    string `db:"tag" json:"tag"`
}
