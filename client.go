package main

import (
  "fmt"
  "net/http"
)

type Client struct {
  baseURL string
  token   string
  httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
  return &Client{
    baseURL: baseURL,
    token: token,
    httpClient: &http.Client{},
  }
}

func (c *Client) newRequest(method, path string) (*http.Request, error) {
  url := fmt.Sprintf("%s%s", c.baseURL, path)
  req, err := http.NewRequest(method, url, nil)
  if err != nil {
    return nil, err
  }
  req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
  req.Header.Set("Content-Type", "application/json")
  return req, nil
}