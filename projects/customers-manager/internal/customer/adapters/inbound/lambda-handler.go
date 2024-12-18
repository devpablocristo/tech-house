package inbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	pkgaws "github.com/devpablocristo/tech-house/pkg/aws"
	awsdefs "github.com/devpablocristo/tech-house/pkg/aws/defs"
	types "github.com/devpablocristo/tech-house/pkg/types"
	utils "github.com/devpablocristo/tech-house/pkg/utils"
	transport "github.com/devpablocristo/tech-house/projects/customers-manager/internal/customer/adapters/inbound/transport"
	ports "github.com/devpablocristo/tech-house/projects/customers-manager/internal/customer/core/ports"
)

type LambdaHandler struct {
	useCases     ports.UseCases
	lambdaClient awsdefs.LambdaClient
}

func NewLambdaHandler(useCases ports.UseCases) (*LambdaHandler, error) {
	stack, err := pkgaws.Bootstrap()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS stack: %w", err)
	}

	lambdaClient := stack.NewLambdaClient()
	if lambdaClient == nil {
		return nil, fmt.Errorf("failed to create Lambda client")
	}

	return &LambdaHandler{
		useCases:     useCases,
		lambdaClient: lambdaClient,
	}, nil
}

func (h *LambdaHandler) HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	switch {
	case request.HTTPMethod == "GET" && request.Resource == "/customers":
		return h.GetCustomers(ctx)
	case request.HTTPMethod == "GET" && request.Resource == "/customers/{id}":
		return h.GetCustomer(ctx, request)
	case request.HTTPMethod == "POST" && request.Resource == "/customers":
		return h.CreateCustomer(ctx, request)
	case request.HTTPMethod == "PUT" && request.Resource == "/customers/{id}":
		return h.UpdateCustomer(ctx, request)
	case request.HTTPMethod == "DELETE" && request.Resource == "/customers/{id}":
		return h.DeleteCustomer(ctx, request)
	case request.HTTPMethod == "GET" && request.Resource == "/customers/kpi":
		return h.GetKPI(ctx)
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNotFound,
			Body:       "Not Found",
		}, nil
	}
}

func (h *LambdaHandler) GetCustomers(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	customers, err := h.useCases.GetCustomers(ctx)
	if err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	response := transport.GetCustomersResponse{
		Customers: transport.DomainListToCustomerJsonList(customers),
	}

	body, err := json.Marshal(response)
	if err != nil {
		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrInternal,
				"Error marshalling response",
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func (h *LambdaHandler) GetCustomer(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	ID, err := strconv.ParseInt(request.PathParameters["id"], 10, 64)
	if err != nil {
		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrInvalidInput,
				"invalid customer ID format",
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	if err := utils.ValidateID(ID); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	customer, err := h.useCases.GetCustomerByID(ctx, ID)
	if err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	response := transport.GetCustomerResponse{
		Customers: *transport.DomainToCustomerJson(customer),
	}

	body, err := json.Marshal(response)
	if err != nil {
		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrInternal,
				"Error marshalling response",
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func (h *LambdaHandler) CreateCustomer(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req transport.CustomerJson
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		errStr := err.Error()
		var message string
		switch {
		case strings.Contains(errStr, "Email' failed on the 'required' tag"):
			message = "invalid email format"
		case strings.Contains(errStr, "Age' failed on the 'required' tag"):
			message = "invalid age"
		case strings.Contains(errStr, "failed on the 'required' tag"):
			message = "missing required field"
		case strings.Contains(errStr, "cannot unmarshal"):
			message = "invalid data type"
		default:
			message = "request cannot be nil"
		}

		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrValidation,
				message,
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	if err := validateRequest(&req); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	if err := h.useCases.CreateCustomer(ctx, transport.CustomerJsonToDomain(&req)); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func (h *LambdaHandler) UpdateCustomer(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	ID, err := strconv.ParseInt(request.PathParameters["id"], 10, 64)
	if err != nil {
		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrInvalidInput,
				"invalid customer ID format",
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	if err := utils.ValidateID(ID); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	var req transport.CustomerJson
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrValidation,
				"invalid request body",
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	if err := validateRequest(&req); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	customer := transport.CustomerJsonToDomain(&req)
	customer.ID = ID

	if err := h.useCases.UpdateCustomer(ctx, customer); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func (h *LambdaHandler) DeleteCustomer(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	ID, err := strconv.ParseInt(request.PathParameters["id"], 10, 64)
	if err != nil {
		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrInvalidInput,
				"invalid customer ID format",
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	if err := utils.ValidateID(ID); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	if err := h.useCases.DeleteCustomer(ctx, ID); err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func (h *LambdaHandler) GetKPI(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	kpi, err := h.useCases.GetKPI(ctx)
	if err != nil {
		apiErr, status := types.NewAPIError(err)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	// Usar directamente el mismo formato que en Gin
	response := transport.ToGetKPIJson(kpi)
	body, err := json.Marshal(response)
	if err != nil {
		apiErr, status := types.NewAPIError(
			types.NewError(
				types.ErrInternal,
				"Error marshalling response",
				err,
			),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Body:       apiErr.Error(),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}