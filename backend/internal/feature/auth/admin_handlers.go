package auth

import (
	"context"
	"errors"
	"net/http"

	"couchverse/internal/httpx"
)

type AdminUsers struct {
	store *Store
}

func NewAdminUsers(st *Store) *AdminUsers {
	return &AdminUsers{store: st}
}

type usersOutput struct{ Body []User }

func (h *AdminUsers) List(ctx context.Context, _ *struct{}) (*usersOutput, error) {
	users, err := h.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &usersOutput{Body: users}, nil
}

// NewUser is an account an admin creates.
type NewUser struct {
	Username    string `json:"username" minLength:"1"`
	DisplayName string `json:"displayName" required:"false" doc:"Defaults to the username."`
	Password    string `json:"password" minLength:"4"`
	Role        string `json:"role" required:"false" enum:"admin,member" default:"member"`
}

type createUserInput struct{ Body NewUser }

type userCreatedOutput struct{ Body *User }

func (h *AdminUsers) Create(ctx context.Context, in *createUserInput) (*userCreatedOutput, error) {
	req := in.Body
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user, err := h.store.CreateUser(ctx, req.Username, req.DisplayName, hash, req.Role)
	if errors.Is(err, ErrUsernameTaken) {
		return nil, httpx.Fail(http.StatusConflict, "conflict", "username already exists")
	}
	if err != nil {
		return nil, err
	}
	return &userCreatedOutput{Body: user}, nil
}

// AdminUserUpdate is a partial update: absent fields keep their value.
type AdminUserUpdate struct {
	DisplayName *string `json:"displayName" required:"false"`
	Role        *string `json:"role" required:"false" enum:"admin,member"`
	Disabled    *bool   `json:"disabled" required:"false" doc:"Disabling an account also signs it out everywhere."`
	Password    *string `json:"password" required:"false" minLength:"4"`
}

type userIDInput struct {
	ID int64 `path:"id"`
}

type updateUserInput struct {
	ID   int64 `path:"id"`
	Body AdminUserUpdate
}

func (h *AdminUsers) Update(ctx context.Context, in *updateUserInput) (*userOutput, error) {
	req := in.Body
	if in.ID == UserFrom(ctx).ID && ((req.Disabled != nil && *req.Disabled) || (req.Role != nil && *req.Role != "admin")) {
		return nil, httpx.BadRequestError("you cannot disable or demote your own account")
	}

	up := UserUpdate{DisplayName: req.DisplayName, Role: req.Role, Disabled: req.Disabled}
	if req.Password != nil {
		hash, err := HashPassword(*req.Password)
		if err != nil {
			return nil, err
		}
		up.PasswordHash = &hash
	}

	user, err := h.store.UpdateUser(ctx, in.ID, up)
	if err != nil {
		return nil, err
	}
	if req.Disabled != nil && *req.Disabled {
		if err := h.store.DeleteUserSessions(ctx, in.ID); err != nil {
			return nil, err
		}
	}
	return &userOutput{Body: user}, nil
}

func (h *AdminUsers) Delete(ctx context.Context, in *userIDInput) (*struct{}, error) {
	if in.ID == UserFrom(ctx).ID {
		return nil, httpx.BadRequestError("you cannot delete your own account")
	}
	return nil, h.store.DeleteUser(ctx, in.ID)
}
