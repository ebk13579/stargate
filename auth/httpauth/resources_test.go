// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package httpauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/stargate/auth"
	"storj.io/stargate/auth/memauth"
)

const minimalAccess = "138CV9Drxrw8ir1XpxcZhk2wnHjhzVjuSZe6yDsNiMZDP8cow9V6sHDYdwgvYoQGgqVvoMnxdWDbpBiEPW5oP7DtPJ5sZx2MVxFrUoZYFfVAgxidW"

func TestResources_URLs(t *testing.T) {
	check := func(method, path string) bool {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer authToken")
		New(nil, "endpoint", "authToken").ServeHTTP(rec, req)
		return rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed
	}

	// check valid paths
	require.True(t, check("POST", "/v1/access"))
	require.True(t, check("GET", "/v1/access/someid"))
	require.True(t, check("PUT", "/v1/access/someid/invalid"))
	require.True(t, check("DELETE", "/v1/access/someid"))

	// check invalid methods
	require.False(t, check("PATCH", "/v1/access"))
	require.False(t, check("PATCH", "/v1/access/someid"))
	require.False(t, check("PATCH", "/v1/access/someid/invalid"))

	// check suffix doesn't match
	require.False(t, check("POST", "/v1/access/extra"))
	require.False(t, check("GET", "/v1/access/someid/extra"))
	require.False(t, check("PUT", "/v1/access/someid/invalid/extra"))
	require.False(t, check("DELETE", "/v1/access/someid/extra"))

	// check misspelling doesn't match
	require.False(t, check("POST", "/v1/access_"))
	require.False(t, check("GET", "/v1/access_/someid"))
	require.False(t, check("DELETE", "/v1/access_/someid"))
	require.False(t, check("PUT", "/v1/access_/someid/invalid"))
	require.False(t, check("PUT", "/v1/access/someid/invalid_"))

	// check trailing slashes are invalid
	require.False(t, check("POST", "/v1/access/"))
	require.False(t, check("GET", "/v1/access/someid/"))
	require.False(t, check("PUT", "/v1/access/someid/invalid/"))
	require.False(t, check("DELETE", "/v1/access/someid/"))
}

func TestResources_CRUD(t *testing.T) {
	exec := func(res http.Handler, method, path, body string) (map[string]interface{}, bool) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer authToken")
		res.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			return nil, false
		}
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		return out, true
	}

	t.Run("CRUD", func(t *testing.T) {
		res := New(auth.NewDatabase(memauth.New()), "endpoint", "authToken")

		// create an access
		createRequest := fmt.Sprintf(`{"access_grant": %q}`, minimalAccess)
		createResult, ok := exec(res, "POST", "/v1/access", createRequest)
		require.True(t, ok)
		require.Equal(t, createResult["endpoint"], "endpoint")
		url := fmt.Sprintf("/v1/access/%s", createResult["access_key_id"])

		// retrieve an access
		fetchResult, ok := exec(res, "GET", url, ``)
		require.True(t, ok)
		require.Equal(t, minimalAccess, fetchResult["access_grant"])
		require.Equal(t, createResult["secret_key"], fetchResult["secret_key"])

		// delete an access
		deleteResult, ok := exec(res, "DELETE", url, ``)
		require.True(t, ok)
		require.Equal(t, map[string]interface{}{}, deleteResult)

		// retrieve fails now
		_, ok = exec(res, "GET", url, ``)
		require.False(t, ok)
	})

	t.Run("Invalidate", func(t *testing.T) {
		res := New(auth.NewDatabase(memauth.New()), "endpoint", "authToken")

		// create an access
		createRequest := fmt.Sprintf(`{"access_grant": %q}`, minimalAccess)
		createResult, ok := exec(res, "POST", "/v1/access", createRequest)
		require.True(t, ok)
		require.Equal(t, createResult["endpoint"], "endpoint")
		url := fmt.Sprintf("/v1/access/%s", createResult["access_key_id"])

		// retrieve an access
		fetchResult, ok := exec(res, "GET", url, ``)
		require.True(t, ok)
		require.Equal(t, minimalAccess, fetchResult["access_grant"])

		// invalidate an access
		invalidateResult, ok := exec(res, "PUT", url+"/invalid", `{"reason": "test"}`)
		require.True(t, ok)
		require.Equal(t, map[string]interface{}{}, invalidateResult)

		// retrieve fails now
		_, ok = exec(res, "GET", url, ``)
		require.False(t, ok)
	})

	t.Run("Public", func(t *testing.T) {
		res := New(auth.NewDatabase(memauth.New()), "endpoint", "authToken")

		// create a public access
		createRequest := fmt.Sprintf(`{"access_grant": %q, "public": true}`, minimalAccess)
		createResult, ok := exec(res, "POST", "/v1/access", createRequest)
		require.True(t, ok)
		require.Equal(t, createResult["endpoint"], "endpoint")
		url := fmt.Sprintf("/v1/access/%s", createResult["access_key_id"])

		// retrieve an access
		fetchResult, ok := exec(res, "GET", url, ``)
		require.True(t, ok)
		require.Equal(t, minimalAccess, fetchResult["access_grant"])
		require.True(t, fetchResult["public"].(bool))
	})
}

func TestResources_Authorization(t *testing.T) {
	res := New(auth.NewDatabase(memauth.New()), "endpoint", "authToken")

	// create an access grant and base url
	createRequest := fmt.Sprintf(`{"access_grant": %q}`, minimalAccess)
	req := httptest.NewRequest("POST", "/v1/access", strings.NewReader(createRequest))
	rec := httptest.NewRecorder()
	res.ServeHTTP(rec, req)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	baseURL := fmt.Sprintf("/v1/access/%s", out["access_key_id"])

	check := func(method, path string) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		res.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}

	// check that these requests are unauthorized
	check("GET", baseURL)
	check("PUT", baseURL+"/invalid")
	check("DELETE", baseURL)
}
