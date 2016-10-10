package mixin

type BeforeCreate interface {
	BeforeCreate() error
}

type BeforeUpdate interface {
	BeforeUpdate(Entity) error
}

type BeforeDelete interface {
	BeforeDelete() error
}

type AfterCreate interface {
	AfterCreate() error
}

type AfterUpdate interface {
	AfterUpdate(Entity) error
}

type AfterDelete interface {
	AfterDelete() error
}
