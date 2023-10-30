package main

import (
	"context"
	"fmt"
	"log"
)

var GradleVersion = "jdk21-alpine"

type GradleService struct {
	Source *Directory
}

func (m *GradleService) WithSource(src *Directory) *GradleService {
	m.Source = src
	return m
}

func (m *GradleService) Build(ctx context.Context) *Container {
	return getGradle(m.Source).Build()
}

func (m *GradleService) Test(ctx context.Context) *Container {
	return getGradle(m.Source).Test()
}

func (m *GradleService) BuildRuntime(ctx context.Context) *Container {
	ctr, err := m.Build(ctx).Sync(ctx)
	if err != nil {
		log.Fatalf("bulid failed: %s", err)
	}

	artifactName, err := m.getArtifactName(ctx)
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
	if src == nil {
		panic("source directory is required. You need to call WithSource before performing actions")
	}

	return dag.Gradle().
		WithVersion(GradleVersion).
		WithSource(src)
}

func (m *GradleService) getArtifactName(ctx context.Context) (string, error) {
	artifact, err := getGradle(m.Source).Task("artifact", []string{"-q"}).Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("could not get artifact name: %s", err)
	}
	return fmt.Sprintf("build/libs/%s", artifact), nil
}
