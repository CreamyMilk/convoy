package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
)

func TestApplicationHandler_GetGroup(t *testing.T) {

	realOrgID := "1234567890"
	fakeOrgID := "12345"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		id         string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "group not found",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			id:         fakeOrgID,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				c, _ := app.cache.(*mocks.MockCache)

				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.ErrGroupNotFound)
			},
		},
		{
			name:       "should_fail_to_count_group_apps",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			id:         fakeOrgID,
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.Any()).Times(1).
					Return(int64(0), errors.New("failed to count group apps"))
			},
		},
		{
			name:       "should_fail_to_count_group_messages",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			id:         fakeOrgID,
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().
					CountGroupMessages(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(0), errors.New("failed to count group messages"))
			},
		},
		{
			name:       "valid group",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			id:         realOrgID,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				c, _ := app.cache.(*mocks.MockCache)

				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().
					CountGroupMessages(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app = provideApplication(ctrl)

			// Arrange
			url := fmt.Sprintf("/api/v1/groups/%s", tc.id)
			req := httptest.NewRequest(tc.method, url, nil)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("groupID", tc.id)

			req = req.Clone(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)
			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}

func TestApplicationHandler_CreateGroup(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		body       *strings.Reader
		dbFn       func(*applicationHandler)
	}{
		{
			name:       "valid group",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,

			body: bodyReader,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					CreateGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "should_fail_to_create_group",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE_2","rate_limit": 3000,"rate_limit_duration": "1m", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					CreateGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("failed"))
			},
		},

		{
			name:       "invalid request - no group name",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group strategy type field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - unsupported group strategy type",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "unsupported", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group interval seconds field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group retry limit field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group header field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": {"hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group hash field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature" }}}`),
		},

		{
			name:       "invalid request - unsupported group hash field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "unsupported" }}}`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app = provideApplication(ctrl)

			// Arrange
			req := httptest.NewRequest(tc.method, "/api/v1/groups", tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			router := buildRoutes(app)

			// Act.
			router.ServeHTTP(w, req)

			// Assert.
			if w.Code != tc.statusCode {
				t.Errorf("want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			w.Body = stripTimestamp(t, "group", w.Body)

			verifyMatch(t, *w)
		})
	}
}

func TestApplicationHandler_UpdateGroup(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	realOrgID := "1234567890"

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		orgID      string
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid group update",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				g.EXPECT().
					UpdateGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
			},
		},
		{
			name:       "invalid request",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			orgID:      realOrgID,
			body:       strings.NewReader(``),
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
			},
		},
		{
			name:       "invalid request - no group name",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			orgID:      realOrgID,
			body:       strings.NewReader(`{"config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
			},
		},
		{
			name:       "should_fail_to_update_group",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			orgID:      realOrgID,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					UpdateGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("failed"))

				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app = provideApplication(ctrl)

			// Arrange
			url := fmt.Sprintf("/api/v1/groups/%s", tc.orgID)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", tc.orgID)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			router := buildRoutes(app)

			// Act.
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_GetGroups(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	realOrgID := "1234567890"

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		groupName  string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid groups",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{
						{
							UID:  realOrgID,
							Name: "sendcash-pay",
						},
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().
					CountGroupMessages(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)
			},
		},
		{
			name:       "should_fail_to_fetch_groups",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return(nil, errors.New("failed"))
			},
		},
		{
			name:       "should_fetch_user_groups",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{
						{
							UID:  realOrgID,
							Name: "sendcash-pay",
						},
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().
					CountGroupMessages(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)
			},
		},
		{
			name:       "should_error_for_invalid_group_access",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			groupName:  "termii",
			method:     http.MethodGet,
			statusCode: http.StatusForbidden,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app := provideApplication(ctrl)

			req := httptest.NewRequest(tc.method, fmt.Sprintf("/api/v1/groups?name=%s", tc.groupName), nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)
			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}

func TestApplicationHandler_DeleteGroup(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	realOrgID := "1234567890"

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE"}`)
	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		orgID      string
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid group delete",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodDelete,
			statusCode: http.StatusOK,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				c, _ := app.cache.(*mocks.MockCache)

				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				g.EXPECT().
					DeleteGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteGroupApps(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().DeleteGroupEvents(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(nil)

			},
		},
		{
			name:       "failed group delete",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodDelete,
			statusCode: http.StatusBadRequest,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				c, _ := app.cache.(*mocks.MockCache)

				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				g.EXPECT().
					DeleteGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("abc"))
			},
		},
		{
			name:       "failed group apps delete",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodDelete,
			statusCode: http.StatusBadRequest,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)

				g.EXPECT().
					DeleteGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteGroupApps(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(errors.New("failed"))
			},
		},
		{
			name:       "failed group events delete",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodDelete,
			statusCode: http.StatusBadRequest,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)

				g.EXPECT().
					DeleteGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteGroupApps(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().DeleteGroupEvents(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(errors.New("failed"))

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app = provideApplication(ctrl)

			// Arrange
			url := fmt.Sprintf("/api/v1/groups/%s", tc.orgID)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", tc.orgID)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			router := buildRoutes(app)

			// Act.
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}
