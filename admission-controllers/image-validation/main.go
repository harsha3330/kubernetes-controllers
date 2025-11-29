package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger Logger

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.PrintInfo("[Middleware] Request Details", map[string]string{
			"method":  r.Method,
			"path":    string(r.URL.Path),
			"address": r.RemoteAddr,
		})
		next.ServeHTTP(w, r)
	})
}

func validateImage(image string) bool {
	privateRegistries := []string{
		"095728565421.dkr.ecr",
	}

	for _, prefix := range privateRegistries {
		if strings.HasPrefix(image, prefix) {
			return true
		}
	}

	// If no prefix matched → public image
	return false
}

func validateDeployment(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var admissionReviewRequest admissionv1.AdmissionReview
	_ = json.NewDecoder(r.Body).Decode(&admissionReviewRequest)
	var deployment appsv1.Deployment
	_ = json.Unmarshal(admissionReviewRequest.Request.Object.Raw, &deployment)

	var images []string
	containers := deployment.Spec.Template.Spec.Containers
	initContainers := deployment.Spec.Template.Spec.InitContainers

	for _, container := range containers {
		images = append(images, container.Image)
	}

	for _, container := range initContainers {
		images = append(images, container.Image)
	}

	validationFlag := true

	for _, image := range images {
		if !validateImage(image) {
			validationFlag = false
			break
		}
	}

	logger.PrintInfo("Validated Deployment Images", map[string]string{
		"requestId":  string(admissionReviewRequest.Request.UID),
		"validation": fmt.Sprintf("%v", validationFlag),
		"deployment": deployment.Name,
		"namespace":  deployment.Namespace,
	})

	admissionResponse := &admissionv1.AdmissionResponse{
		UID:     admissionReviewRequest.Request.UID,
		Allowed: validationFlag,
	}

	responseReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: admissionResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	data, _ := json.Marshal(responseReview)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func health(w http.ResponseWriter, r *http.Request) {
	data := map[string]string{
		"status": "healthy",
	}

	w.Header().Set("Content-Type", "application/json")

	jsonData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "failed to marshal JSON", http.StatusInternalServerError)
		return
	}

	w.Write(jsonData)
}

func main() {
	port := flag.String("port", "8080", "Port to run the HTTP server on")
	flag.Parse()
	logger = *NewLogger(os.Stdout, LevelDebug)
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", health)
	mux.HandleFunc("/validate/deployment", validateDeployment)

	wrapper := loggingMiddleware(mux)
	server := http.Server{
		Addr:    ":" + *port,
		Handler: wrapper,
	}

	log.Printf("Starting server on port %s\n", *port)
	log.Fatal(server.ListenAndServe())
}
