package main

import (
	"context"
	"fmt"
	"log"
	"strings"
)

var GradleVersion = "jdk21-alpine"

type GradleService struct {
	Gradle *Gradle
}

func (m *GradleService) WithSource(src *Directory) *GradleService {
	m.Gradle = getGradle(src)
	return m
}

func (m *GradleService) Build(ctx context.Context) *Container {
	return m.Gradle.Build()
}

func (m *GradleService) Test(ctx context.Context) *Container {
	return m.Gradle.Test()
}

func (m *GradleService) BuildRuntime(ctx context.Context) *Container {
	ctr, err := m.Build(ctx).Sync(ctx)
	if err != nil {
		log.Fatalf("bulid failed: %s", err)
	}
	artifactName, err := getArtifactName(ctx, ctr)
	if err != nil {
		log.Fatalf("could not get artifact name: %s", err)
	}

	jar := ctr.File(artifactName)
	return dag.Container().
		From("amazoncorretto:21.0.1-alpine3.18").
		WithWorkdir("/app").
		WithFile("app.jar", jar).
		WithEntrypoint([]string{"java", "-jar", "app.jar", "--server.port=80", "--spring.profiles.active=default"})
}

func (m *GradleService) Publish(ctx context.Context, tag string) (string, error) {
	return m.BuildRuntime(ctx).Publish(ctx, fmt.Sprintf("services-orders:%s", tag))
}

func (m *GradleService) Service(ctx context.Context) *Service {
	runtime := m.BuildRuntime(ctx)

	return runtime.
		WithEnvVariable("DB_HOST", "mysql").
		WithEnvVariable("DB_PORT", "3306").
		WithServiceBinding("mysql", m.Mysql(ctx)).
		WithExposedPort(80).
		AsService()
}

func (m *GradleService) Mysql(ctx context.Context) *Service {
	return dag.Container().
		From("mysql:8.2.0").
		WithEnvVariable("MYSQL_ROOT_PASSWORD", "gotiendanube").
		WithEnvVariable("MYSQL_DATABASE", "tiendanube").
		WithFile("/docker-entrypoint-initdb.d/db.sql", dag.Host().File("db/db.sql")).
		WithExposedPort(3306).
		AsService()
}

func getGradle(src *Directory) *Gradle {
	return dag.Gradle().
		WithVersion(GradleVersion).
		WithSource(src)
}

func getArtifactName(ctx context.Context, ctr *Container) (string, error) {
	kgradle := ctr.File("build.gradle.kts")
	if kgradle != nil {
		// read contents of build.gradle.kts and join together
		// the description and the version
		return extractArtifactContents(ctx, kgradle)
	}

	return extractArtifactContents(ctx, ctr.File("build.gradle"))
}

func extractArtifactContents(ctx context.Context, f *File) (string, error) {
	if f == nil {
		return "", fmt.Errorf("gradle build file not found")
	}

	contents, err := f.Contents(ctx)
	if err != nil {
		return "", fmt.Errorf("could not read gradle build file: %w", err)
	}

	r := strings.NewReader(contents)

	var description, version string
	if _, err := fmt.Fscanf(r, "description = '%s'", &description, &version); err != nil {
		return "", err
	}

	if description == "" {
		r.Reset(contents)
		if _, err := fmt.Fscanf(r, "description = \"%s\"", &description, &version); err != nil {
			return "", err
		}
	}

	r.Reset(contents)
	if _, err := fmt.Fscanf(r, "version = '%s'", &version); err != nil {
		return "", err
	}

	if version == "" {
		r.Reset(contents)
		if _, err := fmt.Fscanf(r, "version = \"%s\"", &version); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("build/libs/%s-%s.jar", description, version), nil
}
