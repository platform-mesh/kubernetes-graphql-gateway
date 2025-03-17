package resolver

func (r *Service) GetOriginalGroupName(key string) string {
	return r.getOriginalGroupName(key)
}

func (r *Service) GetGroupName(key string) string {
	return r.groupNames[key]
}

func (r *Service) SetGroupNames(names map[string]string) {
	r.groupNames = names
}
