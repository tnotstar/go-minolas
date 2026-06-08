package generators

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"
)

type mockGenerator struct {
	genText string
	genErr  error
}

func (m *mockGenerator) Generate(_ context.Context, _ string, _ ...Option) (*Response, error) {
	return &Response{Text: m.genText}, m.genErr
}

func (m *mockGenerator) Stream(_ context.Context, _ string, _ ...Option) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Text: m.genText}
	close(ch)
	return ch, m.genErr
}

func (m *mockGenerator) Close() error { return nil }

type mockOpener struct {
	id      string
	canOpen bool
	openErr error
	gen     Generator
}

func (m *mockOpener) Id() string { return m.id }

func (m *mockOpener) CanOpen(_ *url.URL) bool { return m.canOpen }

func (m *mockOpener) Open(_ context.Context, _ *url.URL) (Generator, error) {
	return m.gen, m.openErr
}

func TestOpen(t *testing.T) {
	mockGen := &mockGenerator{genText: "hello"}
	errFail := errors.New("fail")

	testCases := []struct {
		name       string
		aiurl      string
		openers    []Opener
		wantGen    Generator
		wantErr    error
		assertions func(t *testing.T, gen Generator, err error)
	}{
		{
			name:    "invalid url returns an error",
			aiurl:   "%#@$",
			wantErr: &url.Error{},
		},
		{
			name:    "no matching opener returns unsupported error",
			aiurl:   "notfound://anything",
			wantErr: ErrUnsupportedOpener,
		},
		{
			name:  "matching opener returns the generator",
			aiurl: "mock://anything",
			openers: []Opener{
				&mockOpener{id: "mock", canOpen: true, gen: mockGen},
			},
			wantGen: mockGen,
		},
		{
			name:  "matching opener returns an error",
			aiurl: "mock://anything",
			openers: []Opener{
				&mockOpener{id: "mock", canOpen: true, openErr: errFail},
			},
			wantErr: errFail,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ResetOpeners()
			for _, op := range tc.openers {
				RegisterOpener(op)
			}

			gen, err := Open(context.Background(), tc.aiurl)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					var urlErr *url.Error
					if _, ok := tc.wantErr.(*url.Error); ok && errors.As(err, &urlErr) {
						// Correctly identified a URL parsing error.
					} else {
						t.Errorf("expected error '%v', got '%v'", tc.wantErr, err)
					}
				}
			} else if err != nil {
				t.Errorf("expected no error, got '%v'", err)
			}
			if gen != tc.wantGen {
				t.Errorf("expected generator '%v', got '%v'", tc.wantGen, gen)
			}
		})
	}
}

func TestRegister_NilOpenerPanics(t *testing.T) {
	ResetOpeners()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil opener, got none")
		}
	}()
	RegisterOpener(nil)
}

func TestRegister_DuplicatePanics(t *testing.T) {
	ResetOpeners()
	op := &mockOpener{id: "dup"}
	RegisterOpener(op)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate opener, got none")
		}
	}()
	RegisterOpener(op)
}

func TestRegister_ValidOpener(t *testing.T) {
	op := &mockOpener{id: "valid"}
	ResetOpeners()
	RegisterOpener(op)

	if got, ok := openers["valid"]; !ok || got != op {
		t.Error("opener not registered correctly")
	}
}

func TestResetOpeners(t *testing.T) {
	ResetOpeners()
	RegisterOpener(&mockOpener{id: "test"})

	if len(openers) != 1 {
		t.Fatal("opener should have been registered")
	}

	ResetOpeners()

	if len(openers) != 0 {
		t.Errorf("expected openers to be empty, but got %d", len(openers))
	}
}

func TestConcurrency(t *testing.T) {
	ResetOpeners()
	// This test is designed to be run with the -race flag
	// to detect data races.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		RegisterOpener(&mockOpener{id: "concurrent", canOpen: true})
	}()
	go func() {
		defer wg.Done()
		_, _ = Open(context.Background(), "concurrent://anything")
	}()
	wg.Wait()
}
