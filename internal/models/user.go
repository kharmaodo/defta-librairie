package models

type UserRole string

const (
	RoleSuperAdminRoot UserRole = "SUPER_ADMIN_ROOT"
	RoleOwnerLibrary   UserRole = "OWNER_LIBRARY"
)

type UserStatus string

const (
	UserStatusActive   UserStatus = "ACTIVE"
	UserStatusDisabled UserStatus = "DISABLED"
	UserStatusLocked   UserStatus = "LOCKED"
)

type User struct {
	ID                  string
	Username            string
	Email               string
	PasswordHash        string
	Role                UserRole
	Status              UserStatus
	CreatedAt           string
	UpdatedAt           string
	FailedLoginAttempts int
	LockedUntil         string
	LibraryID           string
}
