package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/models"
)

func testUser(id string) models.User {
	return models.User{
		ID:          id,
		Email:       id + "@example.com",
		Role:        models.RoleUser,
		AppAccess:   []string{},
		HasMFA:      false,
		DisplayName: "",
	}
}

func TestUserCacheHit(t *testing.T) {
	c := newUserCache(time.Minute)
	c.set("token", testUser("u1"))

	got, ok := c.get("token")
	assert.True(t, ok)
	assert.Equal(t, "u1", got.ID)
}

func TestUserCacheMiss(t *testing.T) {
	c := newUserCache(time.Minute)

	_, ok := c.get("unknown")
	assert.False(t, ok)
}

func TestUserCacheExpiry(t *testing.T) {
	c := newUserCache(time.Nanosecond)
	c.set("token", testUser("u1"))

	time.Sleep(time.Millisecond)

	_, ok := c.get("token")
	assert.False(t, ok)
}

func TestUserCacheDisabled(t *testing.T) {
	c := newUserCache(0)
	c.set("token", testUser("u1"))

	_, ok := c.get("token")
	assert.False(t, ok)
}

func TestUserCacheEvict(t *testing.T) {
	c := newUserCache(time.Minute)
	c.set("token", testUser("u1"))
	c.evict("token")

	_, ok := c.get("token")
	assert.False(t, ok)
}

func TestUserCacheClear(t *testing.T) {
	c := newUserCache(time.Minute)
	c.set("t1", testUser("u1"))
	c.set("t2", testUser("u2"))
	c.clear()

	_, ok1 := c.get("t1")
	_, ok2 := c.get("t2")
	assert.False(t, ok1)
	assert.False(t, ok2)
}
