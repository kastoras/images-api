// Copyright (C) 2026 Nikos Kastoras
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"log"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/kastoras/images-api/internal/config"
	"github.com/kastoras/images-api/internal/domains/health"
	"github.com/kastoras/images-api/internal/domains/jobs"
	"github.com/kastoras/images-api/internal/domains/resize"
	"github.com/kastoras/images-api/internal/middleware"
	"github.com/kastoras/images-api/internal/server"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	api := server.NewAPIServer(cfg)
	router := mux.NewRouter()
	router.Use(middleware.Logging(api.Log))

	// /health is unauthenticated
	health.Register(router, api)

	// All /api/v1 routes require auth
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(middleware.Auth(api))

	resize.Register(apiRouter, api)
	jobs.Register(apiRouter, api)

	if err := api.Start(router); err != nil {
		log.Fatal(err)
	}
}
