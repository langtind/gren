package ui

// initView renders the initialization wizard
func (m Model) initView() string {
	if m.initState == nil {
		return "Initializing..."
	}

	switch m.initState.currentStep {
	case InitStepWelcome:
		return m.renderWelcomeStep()
	case InitStepAnalysis:
		return m.renderAnalysisStep()
	case InitStepRecommendations:
		return m.renderRecommendationsStep()
	case InitStepCustomization:
		return m.renderCustomizationStep()
	case InitStepPreview:
		return m.renderPreviewStep()
	case InitStepCreated:
		return m.renderCreatedStep()
	case InitStepExecuting:
		return m.renderExecutingStep()
	case InitStepComplete:
		return m.renderCompleteStep()
	case InitStepCommitConfirm:
		return m.renderCommitConfirmStep()
	case InitStepFinal:
		return m.renderFinalStep()
	case InitStepAIGenerating:
		return m.renderAIGeneratingStep()
	case InitStepAIResult:
		return m.renderAIResultStep()
	default:
		return "Unknown step"
	}
}
