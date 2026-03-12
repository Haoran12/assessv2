package response

const (
	CodeSuccess = 200

	CodeBadRequestInvalidPayload = 40001
	CodeBadRequestInvalidParam   = 40002
	CodeBadRequestBusinessRule   = 40003

	CodeUnauthorized                  = 40100
	CodeUnauthorizedInvalidCredential = 40101

	CodeForbidden         = 40301
	CodeForbiddenOrgScope = 40302

	CodeNotFound = 40401

	CodeInternal = 50001
)
