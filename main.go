package main

import (
	"net/http"
	"oci-remote-cache/internal/blobs"
	"oci-remote-cache/internal/cache"
	"oci-remote-cache/internal/config"
	"oci-remote-cache/internal/metadata"
	"oci-remote-cache/internal/proxy"
	httputils "oci-remote-cache/internal/transport"

	"github.com/gin-gonic/gin"
)

func LoggHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.Request.URL.Path
		c.Next()
		status := c.Writer.Status()
		println(method, path, status)
	}
}

func main() {
	repoConfigProvider := config.NewRepoConfigInMemory()
	repoConfigProvider.AddRepoConfig(&config.RepoConfig{
		RepoKey:   "gcr.io",
		RemoteURL: "https://gcr.io",
	})
	repoConfigProvider.AddRepoConfig(&config.RepoConfig{
		RepoKey:   "registry.access.redhat.com",
		RemoteURL: "https://registry.access.redhat.com",
	})
	repoConfigProvider.AddRepoConfig(&config.RepoConfig{
		RepoKey:   "mcr.microsoft.com",
		RemoteURL: "https://mcr.microsoft.com",
	})
	repoConfigProvider.AddRepoConfig(&config.RepoConfig{
		RepoKey:   "public.ecr.aws",
		RemoteURL: "https://public.ecr.aws",
	})
	repoConfigProvider.AddRepoConfig(&config.RepoConfig{
		RepoKey:   "docker.io",
		RemoteURL: "https://registry-1.docker.io",
	})
	repoConfigProvider.AddRepoConfig(&config.RepoConfig{
		RepoKey:   "ghcr.io",
		RemoteURL: "https://ghcr.io",
	})
	repoConfigProvider.AddRepoConfig(&config.RepoConfig{
		RepoKey:   "quay.io",
		RemoteURL: "https://quay.io",
	})
	tokenCache := httputils.NewTokenCache()
	blobsStore := blobs.NewFSBlobStore("/tmp/cache/oci")
	metadataStore := metadata.NewFSMetadataStore("/tmp/cache/oci")
	cas := cache.NewCAS("/tmp/cache/oci", blobsStore, metadataStore)
	tokenFetcher := httputils.NewTokenFetcher(tokenCache)
	rt := httputils.NewTransportChain(
		http.DefaultTransport,
		httputils.NewAuthRoundTripper(tokenCache, tokenFetcher),
		//		httputils.NewCachingRoundTripper(cas),
	)
	client := &http.Client{
		Transport: rt,
	}
	proxyHandler := proxy.NewHandler(client, repoConfigProvider, cas)
	r := gin.Default()
	r.Use(httputils.TraceHandler())
	r.Use(func(c *gin.Context) {
		proxyHandler.ServeHTTP(c.Writer, c.Request)
	})
	r.Run(":8080")
}
