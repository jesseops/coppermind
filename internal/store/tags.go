package store

func (s *SQLiteStore) AddTag(workID int64, tag string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO work_tags (work_id, tag) VALUES (?, ?)",
		workID, tag,
	)
	return err
}

func (s *SQLiteStore) RemoveTag(workID int64, tag string) error {
	_, err := s.db.Exec(
		"DELETE FROM work_tags WHERE work_id = ? AND tag = ?",
		workID, tag,
	)
	return err
}

func (s *SQLiteStore) ListTags(workID int64) ([]string, error) {
	rows, err := s.db.Query("SELECT tag FROM work_tags WHERE work_id = ? ORDER BY tag", workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *SQLiteStore) ListAllTags(libraryID int64) ([]TagWithCount, error) {
	rows, err := s.db.Query(`
		SELECT wt.tag, COUNT(*) AS cnt
		FROM work_tags wt
		JOIN works w ON w.id = wt.work_id AND w.library_id = ?
		GROUP BY wt.tag
		ORDER BY wt.tag`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagWithCount
	for rows.Next() {
		var tc TagWithCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tc)
	}
	return tags, rows.Err()
}
