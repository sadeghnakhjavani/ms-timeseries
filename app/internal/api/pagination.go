package api

type PaginationSettings struct {
	DefaultLimit int
	MaxLimit     int
}

func NewPaginationSettings(defaultLimit, maxLimit int) PaginationSettings {
	return PaginationSettings{
		DefaultLimit: defaultLimit,
		MaxLimit:     maxLimit,
	}
}
