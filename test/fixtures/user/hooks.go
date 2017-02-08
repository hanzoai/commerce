package user

// Create
func (u *User) BeforeCreate() error {
	u.C = "before"
	return nil
}

func (u *User) AfterCreate() error {
	u.C = "after"
	return nil
}

// Update
func (u *User) BeforeUpdate(previous *User) error {
	if previous.U != "before" {
		u.U = "before"
	} else {
		u.U = "before2"
	}
	return nil
}

func (u *User) AfterUpdate(previous *User) error {
	if previous.U != "before" {
		u.U = "after"
	} else {
		u.U = "after2"
	}
	return nil
}

// Delete
func (u *User) BeforeDelete() error {
	u.D = "before"
	return nil
}

func (u *User) AfterDelete() error {
	u.D = "after"
	return nil
}
