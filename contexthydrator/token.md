 JWT Structure                                                                                                                                                                                                                                      
                                         
  A JWT has 3 base64url-encoded parts separated by dots:                                                                                                                                                                                             
                                                                                                                                                                                                                                                     
  header.payload.signature                                                                                                                                                                                                                           
                                                            
  Your hydration JWT decoded                                                                                                                                                                                                                         
   
  Header:                                                                                                                                                                                                                                            
  {                                                         
    "alg": "HS256",
    "typ": "JWT"
  }                                                                                                                                                                                                                                                  
   
  Payload:                                                                                                                                                                                                                                           
  {                                                         
    "hyd_token": "k9Xv2mQpL8nRjYwT4hZsA1cBdEfGiKoP3uNqVxWyHz0",
    "iat": 1742860800,                                         
    "exp": 1745452800                                                                                                                                                                                                                                
  }                                                                                                                                                                                                                                                  
                                                                                                                                                                                                                                                     
  Signature:                                                                                                                                                                                                                                         
  HMAC-SHA256(                                              
    base64url(header) + "." + base64url(payload),
    secret                                                                                                                                                                                                                                           
  )                                                                                                                                                                                                                                                  
                                                                                                                                                                                                                                                     
  Wire format (what sits in the cookie):                                                                                                                                                                                                             
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9                                                                                                                                                                                                               
  .eyJoeWRfdG9rZW4iOiJrOVh2Mm1RcEw4blJqWXdUNGhac0ExY0JkRWZHaUtvUDN1TnFWeFd5SHowIiwiaWF0IjoxNzQyODYwODAwLCJleHAiOjE3NDU0NTI4MDB9
  .xK8mP2vQnL9rJyWt5hZsB3cAdEfHiMoN4uOqVxWyIz1                                                                                                                                                                                                       
                                                                                                                                                                                                                                                     
  What each part reveals                                                                                                                                                                                                                             
                                                                                                                                                                                                                                                     
  Part         Encoded?        Secret needed?    Contains                                                                                                                                                                                            
  ──────────── ─────────────── ───────────────── ──────────────────────────────                                                                                                                                                                      
  header       base64url only  no                algorithm, token type                                                                                                                                                                               
  payload      base64url only  no                hyd_token, iat, exp                                                                                                                                                                                 
  signature    base64url       yes (to verify)   tamper detection                                                                                                                                                                                    
                                                                                                                                                                                                                                                     
  Anyone can decode the header and payload — base64url is not encryption. But that's fine here because:                                                                                                                                              
                                                                                                                                                                                                                                                     
  - hyd_token is opaque — decoding it reveals nothing about the user                                                                                                                                                                                 
  - There's no user_id, email, role, or any PII in the payload
  - The signature ensures it was issued by your server                                                                                                                                                                                               
                                                                                                                                                                                                                                                     
  Compare this to what you'd see if you put user data in the payload:                                                                                                                                                                                
                                                                                                                                                                                                                                                     
  // ❌ what NOT to put in payload — readable by anyone                                                                                                                                                                                              
  {                                                                                                                                                                                                                                                  
    "user_id": "abc-123",
    "email": "alice@example.com",                                                                                                                                                                                                                    
    "role": "admin",                                        
    "exp": 1745452800
  }                                                                                                                                                                                                                                                  
   
  // ✓ your payload — opaque, reveals nothing                                                                                                                                                                                                        
  {                                                                                                                                                                                                                                                  
    "hyd_token": "k9Xv2mQpL8nRjYwT4hZsA1cBdEfGiKoP3uNqVxWyHz0",
    "iat": 1742860800,                                                                                                                                                                                                                               
    "exp": 1745452800                                                                                                                                                                                                                                
  }                                                                                                                                                                                                                                                  
                                                                                                                                                                                                                                                     
  Verifying it (what your hydrator does)                                                                                                                                                                                                             
   
  1. split on "."  →  header | payload | signature                                                                                                                                                                                                   
  2. recompute HMAC-SHA256(header + "." + payload, secret)                                                                                                                                                                                           
  3. compare with signature  →  mismatch = tampered, reject                                                                                                                                                                                          
  4. decode payload  →  check exp > now, extract hyd_token                                                                                                                                                                                           
  5. pass hyd_token to backend                                                                                                                                                                                                                       
                                                                                                                                                                                                                                                     
  You can inspect any JWT at https://jwt.io — paste the token, it decodes the header and payload instantly without the secret. This is why PII must never go in the payload.                                                                         
                                                                                                                                                                                                                                                     
❯ how to generate and set the secret is aws                                                                                                                                                                                                          
                                                            
⏺ Step 1: Generate the secret                                                                                                                                                                                                                        
                                                                                                                                                                                                                                                   
  Run this once — generates a cryptographically random 256-bit secret:                                                                                                                                                                               
                                                                                                                                                                                                                                                     
  aws secretsmanager create-secret \                                                                                                                                                                                                                 
    --name "context-hydrator/cookie-secret" \                                                                                                                                                                                                        
    --secret-string "$(openssl rand -base64 32)" \                                                                                                                                                                                                   
    --region us-east-1                                                                                                                                                                                                                               
                                                                                                                                                                                                                                                     
  Or if you prefer to generate and control the value explicitly:                                                                                                                                                                                     
                                                                                                                                                                                                                                                     
  # generate locally first                                                                                                                                                                                                                           
  SECRET=$(openssl rand -base64 32)                                                                                                                                                                                                                  
  echo $SECRET   # save this somewhere safe temporarily                                                                                                                                                                                              
                                                                                                                                                                                                                                                     
  # store in AWS                                                                                                                                                                                                                                     
  aws secretsmanager create-secret \                                                                                                                                                                                                                 
    --name "context-hydrator/cookie-secret" \               
    --secret-string "$SECRET" \                                                                                                                                                                                                                      
    --region us-east-1
                                                                                                                                                                                                                                                     
  Step 2: Rotate it (set a rotation policy)                                                                                                                                                                                                          
                                                                                                                                                                                                                                                     
  aws secretsmanager rotate-secret \                                                                                                                                                                                                                 
    --secret-id "context-hydrator/cookie-secret" \                                                                                                                                                                                                   
    --rotation-rules AutomaticallyAfterDays=90
                                                                                                                                                                                                                                                     
  Step 3: Read it in your Go app at startup                                                                                                                                                                                                          
                                                                                                                                                                                                                                                     
  package config                                                                                                                                                                                                                                     
                                                                                                                                                                                                                                                     
  import (
      "context"                                                                                                                                                                                                                                      
      "encoding/json"                                       
      "fmt"

      "github.com/aws/aws-sdk-go-v2/aws"
      "github.com/aws/aws-sdk-go-v2/config"
      "github.com/aws/aws-sdk-go-v2/service/secretsmanager"                                                                                                                                                                                          
  )                                                                                                                                                                                                                                                  
                                                                                                                                                                                                                                                     
  func LoadCookieSecret(ctx context.Context, secretName string) ([]byte, error) {                                                                                                                                                                    
      cfg, err := config.LoadDefaultConfig(ctx)             
      if err != nil {                                                                                                                                                                                                                                
          return nil, fmt.Errorf("aws config: %w", err)                                                                                                                                                                                              
      }                                                                                                                                                                                                                                              
                                                                                                                                                                                                                                                     
      client := secretsmanager.NewFromConfig(cfg)                                                                                                                                                                                                    
                                                            
      result, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{                                                                                                                                                                 
          SecretId: aws.String(secretName),
      })                                                                                                                                                                                                                                             
      if err != nil {                                       
          return nil, fmt.Errorf("get secret: %w", err)
      }                                                                                                                                                                                                                                              
   
      return []byte(*result.SecretString), nil                                                                                                                                                                                                       
  }                                                         

  Then in your config.Load():                                                                                                                                                                                                                        
   
  // internal/config/config.go                                                                                                                                                                                                                       
  func Load() (*Config, error) {                            
      // ... existing envconfig loading ...
                                                                                                                                                                                                                                                     
      // replace COOKIE_SECRET env var with AWS fetch                                                                                                                                                                                                
      secret, err := LoadCookieSecret(context.Background(), "context-hydrator/cookie-secret")                                                                                                                                                        
      if err != nil {                                                                                                                                                                                                                                
          return nil, fmt.Errorf("load cookie secret: %w", err)
      }                                                                                                                                                                                                                                              
      cfg.CookieSecret = string(secret)                     
                                                                                                                                                                                                                                                     
      return cfg, nil
  }                                                                                                                                                                                                                                                  
                                                            
  Step 4: IAM permission for your service                                                                                                                                                                                                            
   
  Your ECS task / EC2 instance / Lambda needs this policy:                                                                                                                                                                                           
                                                            
  {                                                                                                                                                                                                                                                  
    "Version": "2012-10-17",                                
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "secretsmanager:GetSecretValue"
        ],
        "Resource": "arn:aws:secretsmanager:us-east-1:YOUR_ACCOUNT_ID:secret:context-hydrator/cookie-secret-*"
      }                                                                                                                                                                                                                                              
    ]
  }                                                                                                                                                                                                                                                  
                                                            
  Attach it to your ECS task role or EC2 instance profile — no hardcoded credentials needed.                                                                                                                                                         
   
  Step 5: Local development                                                                                                                                                                                                                          
                                                            
  For local dev, avoid hitting AWS. Use the env var fallback:                                                                                                                                                                                        
   
  func Load() (*Config, error) {                                                                                                                                                                                                                     
      // local dev: COOKIE_SECRET set in .env               
      // production: fetch from AWS                                                                                                                                                                                                                  
      if os.Getenv("AWS_EXECUTION_ENV") != "" || os.Getenv("USE_AWS_SECRETS") == "true" {                                                                                                                                                            
          secret, err := LoadCookieSecret(ctx, "context-hydrator/cookie-secret")                                                                                                                                                                     
          if err != nil {                                                                                                                                                                                                                            
              return nil, err                                                                                                                                                                                                                        
          }                                                 
          cfg.CookieSecret = string(secret)                                                                                                                                                                                                          
      }                                                     
      // else: uses COOKIE_SECRET from .env via envconfig
  }                                                                                                                                                                                                                                                  
   
  .env for local:                                                                                                                                                                                                                                    
  COOKIE_SECRET=local-dev-secret-do-not-use-in-prod         
  USE_AWS_SECRETS=false                            
                                                                                                                                                                                                                                                     
  Summary                                                                                                                                                                                                                                            
                                                                                                                                                                                                                                                     
  openssl rand -base64 32                                                                                                                                                                                                                            
          ↓                                                 
  secretsmanager (encrypted at rest, access-logged, rotatable)
          ↓                                                                                                                                                                                                                                          
  IAM role (no hardcoded keys, scoped to your service only)
          ↓                                                                                                                                                                                                                                          
  fetched at startup → held in memory → never written to disk or logs
                                                                                                                                                                                                                                                     
  Never put the secret in .env.example, environment variables in ECS task definitions as plaintext, or CloudFormation templates. Secrets Manager is the right place for it.      