// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package contexttest provides utilities for testing Web/API contexts with models.
package contexttest

import (
	gocontext "context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func mockRequest(t *testing.T, reqPath string) *http.Request {
	method, path, found := strings.Cut(reqPath, " ")
	if !found {
		method = "GET"
		path = reqPath
	}
	requestURL, err := url.Parse(path)
	assert.NoError(t, err)
	req := &http.Request{Method: method, URL: requestURL, Form: url.Values{}}
	req = req.WithContext(middleware.WithContextData(req.Context()))
	return req
}

type MockContextOption struct {
	Render context.Render
}

// MockContext mock context for unit tests
func MockContext(t *testing.T, reqPath string, opts ...MockContextOption) (*context.Context, *httptest.ResponseRecorder) {
	var opt MockContextOption
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Render == nil {
		opt.Render = &MockRender{}
	}
	resp := httptest.NewRecorder()
	req := mockRequest(t, reqPath)
	base, baseCleanUp := context.NewBaseContext(resp, req)
	_ = baseCleanUp // during test, it doesn't need to do clean up. TODO: this can be improved later
	base.Data = middleware.GetContextData(req.Context())
	base.Locale = &translation.MockLocale{}

	ctx := context.NewWebContext(base, opt.Render, nil)

	chiCtx := chi.NewRouteContext()
	ctx.Base.AppendContextValue(chi.RouteCtxKey, chiCtx)
	return ctx, resp
}

// MockAPIContext mock context for unit tests
func MockAPIContext(t *testing.T, reqPath string) (*context.APIContext, *httptest.ResponseRecorder) {
	resp := httptest.NewRecorder()
	req := mockRequest(t, reqPath)
	base, baseCleanUp := context.NewBaseContext(resp, req)
	base.Data = middleware.GetContextData(req.Context())
	base.Locale = &translation.MockLocale{}
	ctx := &context.APIContext{Base: base}
	_ = baseCleanUp // during test, it doesn't need to do clean up. TODO: this can be improved later

	chiCtx := chi.NewRouteContext()
	ctx.Base.AppendContextValue(chi.RouteCtxKey, chiCtx)
	return ctx, resp
}

// LoadRepo load a repo into a test context.
func LoadRepo(t *testing.T, ctx gocontext.Context, repoID int64) {
	var doer *user_model.User
	repo := &context.Repository{}
	switch ctx := ctx.(type) {
	case *context.Context:
		ctx.Repo = repo
		doer = ctx.Doer
	case *context.APIContext:
		ctx.Repo = repo
		doer = ctx.Doer
	default:
		assert.FailNow(t, "context is not *context.Context or *context.APIContext")
	}

	repo.Repository = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	var err error
	repo.Owner, err = user_model.GetUserByID(ctx, repo.Repository.OwnerID)
	assert.NoError(t, err)
	repo.RepoLink = repo.Repository.Link()
	repo.Permission, err = access_model.GetUserRepoPermission(ctx, repo.Repository, doer)
	assert.NoError(t, err)
}

// LoadRepoCommit loads a repo's commit into a test context.
func LoadRepoCommit(t *testing.T, ctx gocontext.Context) {
	var repo *context.Repository
	switch ctx := ctx.(type) {
	case *context.Context:
		repo = ctx.Repo
	case *context.APIContext:
		repo = ctx.Repo
	default:
		assert.FailNow(t, "context is not *context.Context or *context.APIContext")
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, repo.Repository)
	assert.NoError(t, err)
	defer gitRepo.Close()
	branch, err := gitRepo.GetHEADBranch()
	assert.NoError(t, err)
	assert.NotNil(t, branch)
	if branch != nil {
		repo.Commit, err = gitRepo.GetBranchCommit(branch.Name)
		assert.NoError(t, err)
	}
}

// LoadUser load a user into a test context
func LoadUser(t *testing.T, ctx gocontext.Context, userID int64) {
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
	switch ctx := ctx.(type) {
	case *context.Context:
		ctx.Doer = doer
	case *context.APIContext:
		ctx.Doer = doer
	default:
		assert.FailNow(t, "context is not *context.Context or *context.APIContext")
	}
}

// LoadGitRepo load a git repo into a test context. Requires that ctx.Repo has
// already been populated.
func LoadGitRepo(t *testing.T, ctx *context.Context) {
	assert.NoError(t, ctx.Repo.Repository.LoadOwner(ctx))
	var err error
	ctx.Repo.GitRepo, err = gitrepo.OpenRepository(ctx, ctx.Repo.Repository)
	assert.NoError(t, err)
}

type MockRender struct{}

func (tr *MockRender) TemplateLookup(tmpl string, _ gocontext.Context) (templates.TemplateExecutor, error) {
	return nil, nil
}

func (tr *MockRender) HTML(w io.Writer, status int, _ string, _ any, _ gocontext.Context) error {
	if resp, ok := w.(http.ResponseWriter); ok {
		resp.WriteHeader(status)
	}
	return nil
}
