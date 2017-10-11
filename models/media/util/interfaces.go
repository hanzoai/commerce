package util

type BelongsToParentMedia interface {
	GetParentMediaId() string
}

type HasMedias interface {
	GetMediaSearchFieldAndIds() (string, []string)
}
