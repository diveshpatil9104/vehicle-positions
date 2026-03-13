package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_GetUserByEmail_Found(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.pool.Exec(ctx, "DELETE FROM users WHERE email = 'testuser@example.com'")
	require.NoError(t, err)

	_, err = store.pool.Exec(ctx,
		`INSERT INTO users (name, email, password_hash, role) VALUES ($1, $2, $3, $4)`,
		"Test User",
		"testuser@example.com",
		"$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi",
		"driver",
	)
	require.NoError(t, err)

	user, err := store.GetUserByEmail(ctx, "testuser@example.com")
	require.NoError(t, err)

	assert.Equal(t, "testuser@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "driver", user.Role)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotZero(t, user.ID)
}

func TestStore_GetUserByEmail_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	user, err := store.GetUserByEmail(ctx, "nobody@example.com")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.True(t, errors.Is(err, pgx.ErrNoRows), "error must wrap pgx.ErrNoRows so auth handler can distinguish not-found from database failures")
}
