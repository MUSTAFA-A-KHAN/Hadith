package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ChatState struct {
	UseCustomBg      bool          `json:"use_custom_bg"`
	UseClassicArabic bool          `json:"use_classic_arabic"`
	ScheduleInterval time.Duration `json:"schedule_interval"`
	LastSentAt       time.Time     `json:"last_sent_at"`
}

type StateManager struct {
	filePath string
	mu       sync.RWMutex
	data     map[int64]*ChatState
}

func NewStateManager(filePath string) *StateManager {
	sm := &StateManager{
		filePath: filePath,
		data:     make(map[int64]*ChatState),
	}
	sm.Load()
	return sm
}

func (sm *StateManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	b, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(b, &sm.data)
}

func (sm *StateManager) Save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	dir := filepath.Dir(sm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(sm.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sm.filePath, b, 0644)
}

func (sm *StateManager) GetChatState(chatID int64) *ChatState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	state, ok := sm.data[chatID]
	if !ok {
		return nil
	}

	// Return a copy so the caller can't mutate the internal pointer without Save()
	copyState := *state
	return &copyState
}

func (sm *StateManager) SetChatState(chatID int64, state *ChatState) error {
	sm.mu.Lock()
	sm.data[chatID] = state
	sm.mu.Unlock()

	return sm.Save()
}

func (sm *StateManager) GetAll() map[int64]*ChatState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	copyData := make(map[int64]*ChatState, len(sm.data))
	for k, v := range sm.data {
		s := *v
		copyData[k] = &s
	}
	return copyData
}
