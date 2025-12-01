package client

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
)

// GraphQLClient represents a client for interacting with GraphQL APIs.
type GraphQLClient struct {
	client *graphql.Client
}

// NewGraphQLClient creates and returns a new GraphQLClient for the specified endpoint.
func NewGraphQLClient(endpoint string) *GraphQLClient {
	client := graphql.NewClient(endpoint)
	return &GraphQLClient{client: client}
}

// Query executes a GraphQL query and populates the response into the provided interface.
func (g *GraphQLClient) Query(query string, response interface{}) error {
	req := graphql.NewRequest(query)
	err := g.client.Run(context.Background(), req, response)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}
	return nil
}
