// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

//go:generate go-bindata-assetfs -pkg swagger -prefix ../ ../swagger-ui
//go:generate swagger generate spec -m -b ../ -o ../swagger-ui/swagger.json
//go:generate go-bindata-assetfs -pkg swagger -prefix ../ ../swagger-ui/...

package swagger

import (
	"net/http"
	"strings"
)

// NewSwaggerUI returns an HTTP handler which serves swagger UI assets built
// into the binary itself using a virtual fs courtesy of go-bindata-assetfs.
func NewSwaggerUI(swaggerPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == swaggerPath ||
			(r.URL.Path == swaggerPath+"/" && r.URL.Query().Get("url") == "") {
			http.Redirect(w, r, swaggerPath+"/?url=swagger.json", http.StatusFound)
			return
		}

		if strings.Index(r.URL.Path, swaggerPath) == 0 {
			http.StripPrefix(swaggerPath+"/", http.FileServer(assetFS())).ServeHTTP(w, r)
			return
		}
	})
}
