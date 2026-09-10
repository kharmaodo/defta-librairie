package models

type LibraryStatus string

const (
	LibraryStatusActive   LibraryStatus = "ACTIVE"
	LibraryStatusDisabled LibraryStatus = "DISABLED"
)

type Library struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Status      LibraryStatus `json:"status"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
}

type OwnerAccount struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email,omitempty"`
	Status    UserStatus `json:"status"`
	CreatedAt string     `json:"createdAt"`
	UpdatedAt string     `json:"updatedAt"`
	Library   Library    `json:"library"`
}

type OwnerCreateInput struct {
	Username           string
	Email              string
	Password           string
	LibraryName        string
	LibraryDescription string
}

type OwnerUpdateInput struct {
	Username           *string
	Email              *string
	Password           *string
	Status             *UserStatus
	LibraryName        *string
	LibraryDescription *string
	LibraryStatus      *LibraryStatus
}
