package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrAssignmentExists   = errors.New("assignment already exists")
	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrUserNotFoundFK     = errors.New("user not found")
	ErrVehicleNotFoundFK  = errors.New("vehicle not found")
)

// AssignmentResponse is the API representation of a user-vehicle assignment.
type AssignmentResponse struct {
	UserID    int64     `json:"user_id"`
	VehicleID string    `json:"vehicle_id"`
	CreatedAt time.Time `json:"created_at"`
}

// AssignmentCreator creates a user-vehicle assignment.
type AssignmentCreator interface {
	CreateAssignment(ctx context.Context, userID int64, vehicleID string) (*AssignmentResponse, error)
}

// AssignmentDeleter removes a user-vehicle assignment.
type AssignmentDeleter interface {
	DeleteAssignment(ctx context.Context, userID int64, vehicleID string) error
}

// AssignmentListerByUser lists assignments for a given user.
type AssignmentListerByUser interface {
	ListAssignmentsByUser(ctx context.Context, userID int64) ([]AssignmentResponse, error)
}

// AssignmentListerByVehicle lists assignments for a given vehicle.
type AssignmentListerByVehicle interface {
	ListAssignmentsByVehicle(ctx context.Context, vehicleID string) ([]AssignmentResponse, error)
}

// CreateAssignment inserts a user-vehicle assignment and returns the DB-generated created_at.
func (s *Store) CreateAssignment(ctx context.Context, userID int64, vehicleID string) (*AssignmentResponse, error) {
	query := `
		INSERT INTO user_vehicles (user_id, vehicle_id)
		VALUES ($1, $2)
		RETURNING created_at
	`
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, query, userID, vehicleID).Scan(&createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAssignmentExists
		}
		if isFKViolation(err) {
			switch fkConstraintName(err) {
			case "user_vehicles_user_id_fkey":
				return nil, ErrUserNotFoundFK
			case "user_vehicles_vehicle_id_fkey":
				return nil, ErrVehicleNotFoundFK
			}
			return nil, fmt.Errorf("create assignment: foreign key violation: %w", err)
		}
		return nil, fmt.Errorf("create assignment: %w", err)
	}

	return &AssignmentResponse{
		UserID:    userID,
		VehicleID: vehicleID,
		CreatedAt: createdAt,
	}, nil
}

// DeleteAssignment removes a user-vehicle assignment. Returns ErrAssignmentNotFound if no row matched.
func (s *Store) DeleteAssignment(ctx context.Context, userID int64, vehicleID string) error {
	query := `DELETE FROM user_vehicles WHERE user_id = $1 AND vehicle_id = $2`
	ct, err := s.pool.Exec(ctx, query, userID, vehicleID)
	if err != nil {
		return fmt.Errorf("delete assignment: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}

// ListAssignmentsByUser returns all assignments for a user, ordered by created_at DESC.
func (s *Store) ListAssignmentsByUser(ctx context.Context, userID int64) ([]AssignmentResponse, error) {
	query := `
		SELECT user_id, vehicle_id, created_at
		FROM user_vehicles
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	return s.scanAssignments(ctx, query, userID)
}

// ListAssignmentsByVehicle returns all assignments for a vehicle, ordered by created_at DESC.
func (s *Store) ListAssignmentsByVehicle(ctx context.Context, vehicleID string) ([]AssignmentResponse, error) {
	query := `
		SELECT user_id, vehicle_id, created_at
		FROM user_vehicles
		WHERE vehicle_id = $1
		ORDER BY created_at DESC
	`
	return s.scanAssignments(ctx, query, vehicleID)
}

func (s *Store) scanAssignments(ctx context.Context, query string, arg any) ([]AssignmentResponse, error) {
	rows, err := s.pool.Query(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	defer rows.Close()

	assignments := make([]AssignmentResponse, 0)
	for rows.Next() {
		var a AssignmentResponse
		if err := rows.Scan(&a.UserID, &a.VehicleID, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		assignments = append(assignments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assignments: %w", err)
	}

	return assignments, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isFKViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func fkConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

// Ensure Store satisfies the assignment interfaces at compile time.
var _ AssignmentCreator = (*Store)(nil)
var _ AssignmentDeleter = (*Store)(nil)
var _ AssignmentListerByUser = (*Store)(nil)
var _ AssignmentListerByVehicle = (*Store)(nil)
