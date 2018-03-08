package plan

import "hanzo.io/util/event"

// Hooks
func (p *Plan) AfterCreate() error {
	return event.Emit(p.Context(), p.Namespace(), "plan.created", p)
}

func (p *Plan) AfterUpdate(previous *Plan) error {
	return event.Emit(p.Context(), p.Namespace(), "plan.updated", p)
}

func (p *Plan) AfterDelete() error {
	return event.Emit(p.Context(), p.Namespace(), "plan.deleted", p)
}
