// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

// Rancher Management Service
//
// The purpose of this project is to provide a boilerplate starter for a
// microservice which will sit within the Rancher environment performing
// management operations against running containers, bridging the
// the overlay network for choice operations.
//
// It is designed to be an idiomatic example service complete with
// 12-factorish microservice ecosystem bells and whistles e.g.:
// distributed tracing, structured logging, instrumentation, discovery, auth,
// multiple transports, self-hosted swagger UI, self-generated swagger spec etc.
//
// See the project's README.md for more information.
//
// Terms Of Service:
//
// There are no TOS at this moment, use at your own risk.
//
//     Version: MAKEFILE_REPLACED_VERSION
//     License: BSD-3-Clause https://opensource.org/licenses/BSD-3-Clause
//     Contact: Martin Baillie <martin.t.baillie@gmail.com>
//
//     Schemes: http, https
//
//	   Basepath: /rms/v1
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// swagger:meta
package swagger
