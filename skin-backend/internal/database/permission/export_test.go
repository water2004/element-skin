package permission

func (s *Store) SetTestConn(q Querier) {
	s.q = q
}
