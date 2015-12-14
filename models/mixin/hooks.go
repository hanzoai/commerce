package mixin

type BeforeCreate interface {
	BeforeCreate() error
}

type BeforeUpdate interface {
	BeforeUpdate() error
}

type BeforeDelete interface {
	BeforeDelete() error
}

type AfterCreate interface {
	AfterCreate() error
}

type AfterUpdate interface {
	AfterUpdate() error
}

type AfterDelete interface {
	AfterDelete() error
}
