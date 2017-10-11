package util

type BelongsToParentCopy interface {
	GetParentCopyId() string
}

type HasCopies interface {
	GetCopySearchFieldAndIds() (string, []string)
}
