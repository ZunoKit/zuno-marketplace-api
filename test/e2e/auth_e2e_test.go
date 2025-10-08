package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

func (suite *E2ETestSuite) TestAuthenticationFlow() {
	// Test complete authentication flow from end to end

	// Step 1: Request nonce
	nonceReq := map[string]interface{}{
		"query": `
			mutation RequestNonce($address: String!) {
				requestNonce(address: $address) {
					nonce
					expiresAt
				}
			}
		`,
		"variables": map[string]string{
			"address": "0x742d35cc6634c0532925a3b844bc9e7595f0b0bb",
		},
	}

	body, _ := json.Marshal(nonceReq)
	resp, err := http.Post(suite.baseURL+"/graphql", "application/json", bytes.NewBuffer(body))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode)

	var nonceResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&nonceResp)
	resp.Body.Close()

	// Extract nonce
	data := nonceResp["data"].(map[string]interface{})
	requestNonce := data["requestNonce"].(map[string]interface{})
	nonce := requestNonce["nonce"].(string)
	suite.NotEmpty(nonce)

	// Step 2: Sign message with wallet (mocked)
	signature := "0xmocked_signature_for_testing"
	message := suite.createSIWEMessage(nonce)

	// Step 3: Verify signature and authenticate
	authReq := map[string]interface{}{
		"query": `
			mutation Authenticate($message: String!, $signature: String!) {
				authenticate(message: $message, signature: $signature) {
					accessToken
					refreshToken
					user {
						id
						wallets {
							address
							chainId
						}
					}
				}
			}
		`,
		"variables": map[string]string{
			"message":   message,
			"signature": signature,
		},
	}

	body, _ = json.Marshal(authReq)
	resp, err = http.Post(suite.baseURL+"/graphql", "application/json", bytes.NewBuffer(body))
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&authResp)
	resp.Body.Close()

	// Verify tokens received
	data = authResp["data"].(map[string]interface{})
	authenticate := data["authenticate"].(map[string]interface{})
	suite.NotEmpty(authenticate["accessToken"])
	suite.NotEmpty(authenticate["refreshToken"])
}

func (suite *E2ETestSuite) TestUserProfileCreation() {
	// Test user profile creation after authentication

	// First authenticate
	token := suite.authenticateUser("0x742d35cc6634c0532925a3b844bc9e7595f0b0bb")

	// Update profile
	profileReq := map[string]interface{}{
		"query": `
			mutation UpdateProfile($input: UpdateProfileInput!) {
				updateProfile(input: $input) {
					id
					username
					displayName
					bio
					avatarUrl
				}
			}
		`,
		"variables": map[string]interface{}{
			"input": map[string]string{
				"username":    "testuser",
				"displayName": "Test User",
				"bio":         "NFT enthusiast",
			},
		},
	}

	body, _ := json.Marshal(profileReq)
	req, _ := http.NewRequest("POST", suite.baseURL+"/graphql", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode)

	var profileResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&profileResp)
	resp.Body.Close()

	// Verify profile updated
	data := profileResp["data"].(map[string]interface{})
	profile := data["updateProfile"].(map[string]interface{})
	suite.Equal("testuser", profile["username"])
	suite.Equal("Test User", profile["displayName"])
}

func (suite *E2ETestSuite) createSIWEMessage(nonce string) string {
	// Create a Sign-In with Ethereum message
	return `example.com wants you to sign in with your Ethereum account:
0x742d35cc6634c0532925a3b844bc9e7595f0b0bb

Sign in to NFT Marketplace

URI: https://example.com
Version: 1
Chain ID: 1
Nonce: ` + nonce + `
Issued At: ` + time.Now().Format(time.RFC3339)
}

func (suite *E2ETestSuite) authenticateUser(address string) string {
	// Helper function to authenticate and return token
	// Implementation would go through full auth flow
	return "mocked_jwt_token"
}
