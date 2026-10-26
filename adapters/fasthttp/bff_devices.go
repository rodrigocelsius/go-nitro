package fasthttp

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// DeviceStore persists mobile devices for MobileBFFConfig.EnableDeviceTracking.
// The default is an in-memory store shared by the process; plug a database
// implementation through MobileBFFConfig.Devices for multi-replica setups.
type DeviceStore interface {
	// Save upserts a device (keyed by DeviceID).
	Save(ctx context.Context, d domain.DeviceInfo) error
	// Touch updates LastSeenAt and binds the device to userID when not yet bound.
	Touch(ctx context.Context, deviceID, userID string) error
	// Delete removes a device.
	Delete(ctx context.Context, deviceID string) error
	// List returns the user's devices, most recently seen first.
	List(ctx context.Context, userID string) ([]domain.DeviceInfo, error)
	// Get returns a device (ok=false when unknown).
	Get(ctx context.Context, deviceID string) (domain.DeviceInfo, bool, error)
}

// MemoryDeviceStore is an in-process DeviceStore. maxPerUser > 0 evicts the
// least recently seen devices of a user beyond the limit.
type MemoryDeviceStore struct {
	mu         sync.RWMutex
	devices    map[string]domain.DeviceInfo
	maxPerUser int
}

// NewMemoryDeviceStore creates an in-memory device store.
func NewMemoryDeviceStore(maxPerUser int) *MemoryDeviceStore {
	return &MemoryDeviceStore{devices: make(map[string]domain.DeviceInfo), maxPerUser: maxPerUser}
}

// Save implements DeviceStore.
func (s *MemoryDeviceStore) Save(_ context.Context, d domain.DeviceInfo) error {
	if d.DeviceID == "" {
		return domain.NewBadRequest("device_id is required")
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev, ok := s.devices[d.DeviceID]; ok {
		if d.UserID == "" {
			d.UserID = prev.UserID
		}
		if d.CreatedAt.IsZero() {
			d.CreatedAt = prev.CreatedAt
		}
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	d.LastSeenAt = now
	s.devices[d.DeviceID] = d
	s.enforceLimitLocked(d.UserID)
	return nil
}

// Touch implements DeviceStore.
func (s *MemoryDeviceStore) Touch(_ context.Context, deviceID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[deviceID]
	if !ok {
		d = domain.DeviceInfo{DeviceID: deviceID, CreatedAt: time.Now()}
	}
	if d.UserID == "" {
		d.UserID = userID
	}
	d.LastSeenAt = time.Now()
	s.devices[deviceID] = d
	s.enforceLimitLocked(d.UserID)
	return nil
}

// Delete implements DeviceStore.
func (s *MemoryDeviceStore) Delete(_ context.Context, deviceID string) error {
	s.mu.Lock()
	delete(s.devices, deviceID)
	s.mu.Unlock()
	return nil
}

// Get implements DeviceStore.
func (s *MemoryDeviceStore) Get(_ context.Context, deviceID string) (domain.DeviceInfo, bool, error) {
	s.mu.RLock()
	d, ok := s.devices[deviceID]
	s.mu.RUnlock()
	return d, ok, nil
}

// List implements DeviceStore.
func (s *MemoryDeviceStore) List(_ context.Context, userID string) ([]domain.DeviceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userDevicesLocked(userID), nil
}

func (s *MemoryDeviceStore) userDevicesLocked(userID string) []domain.DeviceInfo {
	out := make([]domain.DeviceInfo, 0, 4)
	if userID == "" {
		return out
	}
	for _, d := range s.devices {
		if d.UserID == userID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeenAt.After(out[j].LastSeenAt) })
	return out
}

func (s *MemoryDeviceStore) enforceLimitLocked(userID string) {
	if s.maxPerUser <= 0 || userID == "" {
		return
	}
	devs := s.userDevicesLocked(userID)
	for _, d := range devs[min(len(devs), s.maxPerUser):] {
		delete(s.devices, d.DeviceID)
	}
}

// defaultDevices backs configs built without Devices (shared by all handlers
// of the process, so login / middleware / logout see the same devices).
var (
	defaultDevicesOnce sync.Once
	defaultDevices     *MemoryDeviceStore
)

func deviceStore(cfg MobileBFFConfig) DeviceStore {
	if cfg.Devices != nil {
		return cfg.Devices
	}
	defaultDevicesOnce.Do(func() { defaultDevices = NewMemoryDeviceStore(cfg.MaxDevicesPerUser) })
	return defaultDevices
}

func logDeviceErr(op string, err error) {
	if err != nil {
		log.Printf("nitro: mobile bff device %s: %v", op, err)
	}
}

// MobileListDevicesHandler lists the authenticated user's devices
// (GET {prefix}/devices; requires MobileBFFMiddleware).
func MobileListDevicesHandler(cfg MobileBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}
		userID, _ := fc.Get("user_id").(string)
		if userID == "" {
			return domain.NewUnauthorized("missing user")
		}
		devs, err := deviceStore(cfg).List(c.Context(), userID)
		if err != nil {
			return domain.NewInternal(err)
		}
		return fc.JSON(200, devs)
	}
}

// MobileRemoveDeviceHandler removes one of the authenticated user's devices
// (DELETE {prefix}/devices/:id; requires MobileBFFMiddleware). Unknown or
// foreign devices answer 404.
func MobileRemoveDeviceHandler(cfg MobileBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}
		userID, _ := fc.Get("user_id").(string)
		id := c.Param("id")
		store := deviceStore(cfg)
		d, ok, err := store.Get(c.Context(), id)
		if err != nil {
			return domain.NewInternal(err)
		}
		if !ok || d.UserID != userID {
			return domain.NewNotFound("device not found")
		}
		if err := store.Delete(c.Context(), id); err != nil {
			return domain.NewInternal(err)
		}
		return domain.RespNoContent()
	}
}
