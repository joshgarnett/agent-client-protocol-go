package acp

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// ContentBuilder provides a fluent interface for building content blocks.
type ContentBuilder struct {
	blocks []api.ContentBlock
}

// NewContentBuilder creates a new content builder.
func NewContentBuilder() *ContentBuilder {
	return &ContentBuilder{
		blocks: make([]api.ContentBlock, 0),
	}
}

// AddText adds a text content block.
func (cb *ContentBuilder) AddText(text string) *ContentBuilder {
	block := api.ContentBlock{
		Type: api.ContentBlockTypeText,
		Text: &api.ContentBlockText{
			Text: text,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// AddTextWithAnnotations adds a text content block with annotations.
func (cb *ContentBuilder) AddTextWithAnnotations(text string, annotations interface{}) *ContentBuilder {
	block := api.ContentBlock{
		Type: api.ContentBlockTypeText,
		Text: &api.ContentBlockText{
			Text:        text,
			Annotations: annotations,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// AddImage adds an image content block with base64 encoded data.
func (cb *ContentBuilder) AddImage(data []byte, mimeType string) *ContentBuilder {
	encodedData := base64.StdEncoding.EncodeToString(data)
	block := api.ContentBlock{
		Type: api.ContentBlockTypeImage,
		Image: &api.ContentBlockImage{
			Data:     encodedData,
			Mimetype: mimeType,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// AddImageURI adds an image content block with a URI reference.
func (cb *ContentBuilder) AddImageURI(uri string, mimeType string) *ContentBuilder {
	block := api.ContentBlock{
		Type: api.ContentBlockTypeImage,
		Image: &api.ContentBlockImage{
			Uri:      uri,
			Mimetype: mimeType,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// AddAudio adds an audio content block with base64 encoded data.
func (cb *ContentBuilder) AddAudio(data []byte, mimeType string) *ContentBuilder {
	encodedData := base64.StdEncoding.EncodeToString(data)
	block := api.ContentBlock{
		Type: api.ContentBlockTypeAudio,
		Audio: &api.ContentBlockAudio{
			Data:     encodedData,
			Mimetype: mimeType,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// AddResource adds a resource content block.
func (cb *ContentBuilder) AddResource(resource *api.EmbeddedResourceResource) *ContentBuilder {
	block := api.ContentBlock{
		Type: api.ContentBlockTypeResource,
		Resource: &api.ContentBlockResource{
			Resource: resource,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// AddResourceLink adds a resource link content block.
func (cb *ContentBuilder) AddResourceLink(uri string, name string) *ContentBuilder {
	block := api.ContentBlock{
		Type: api.ContentBlockTypeResourceLink,
		ResourceLink: &api.ContentBlockResourceLink{
			Uri:  uri,
			Name: name,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// AddResourceLinkFull adds a resource link content block with all fields.
func (cb *ContentBuilder) AddResourceLinkFull(uri, name string, title, description,
	mimeType interface{}, size interface{}) *ContentBuilder {
	block := api.ContentBlock{
		Type: api.ContentBlockTypeResourceLink,
		ResourceLink: &api.ContentBlockResourceLink{
			Uri:         uri,
			Name:        name,
			Title:       title,
			Description: description,
			Mimetype:    mimeType,
			Size:        size,
		},
	}
	cb.blocks = append(cb.blocks, block)
	return cb
}

// Build returns the constructed content blocks.
func (cb *ContentBuilder) Build() []api.ContentBlock {
	return cb.blocks
}

// BuildInterface returns the content blocks as []interface{} for API compatibility.
func (cb *ContentBuilder) BuildInterface() []interface{} {
	result := make([]interface{}, len(cb.blocks))
	for i, block := range cb.blocks {
		result[i] = block
	}
	return result
}

// Content creation helpers

// NewTextContent creates a simple text content block.
func NewTextContent(text string) api.ContentBlock {
	return api.ContentBlock{
		Type: api.ContentBlockTypeText,
		Text: &api.ContentBlockText{
			Text: text,
		},
	}
}

// NewImageContent creates an image content block from raw data.
func NewImageContent(data []byte, mimeType string) api.ContentBlock {
	encodedData := base64.StdEncoding.EncodeToString(data)
	return api.ContentBlock{
		Type: api.ContentBlockTypeImage,
		Image: &api.ContentBlockImage{
			Data:     encodedData,
			Mimetype: mimeType,
		},
	}
}

// NewAudioContent creates an audio content block from raw data.
func NewAudioContent(data []byte, mimeType string) api.ContentBlock {
	encodedData := base64.StdEncoding.EncodeToString(data)
	return api.ContentBlock{
		Type: api.ContentBlockTypeAudio,
		Audio: &api.ContentBlockAudio{
			Data:     encodedData,
			Mimetype: mimeType,
		},
	}
}

// NewResourceLink creates a resource link content block.
func NewResourceLink(uri, name string) api.ContentBlock {
	return api.ContentBlock{
		Type: api.ContentBlockTypeResourceLink,
		ResourceLink: &api.ContentBlockResourceLink{
			Uri:  uri,
			Name: name,
		},
	}
}

// Content validation helpers

// ValidateContentBlock validates a single content block.
func ValidateContentBlock(block api.ContentBlock) error {
	if !block.Type.IsValid() {
		return fmt.Errorf("invalid content block type: %s", string(block.Type))
	}

	switch block.Type {
	case api.ContentBlockTypeText:
		if block.Text == nil {
			return errors.New("text field is required for text content block")
		}
	case api.ContentBlockTypeImage:
		if block.Image == nil {
			return errors.New("image field is required for image content block")
		}
		if block.Image.Data == "" && block.Image.Uri == nil {
			return errors.New("either data or uri is required for image content block")
		}
	case api.ContentBlockTypeAudio:
		if block.Audio == nil {
			return errors.New("audio field is required for audio content block")
		}
		if block.Audio.Data == "" {
			return errors.New("data is required for audio content block")
		}
	case api.ContentBlockTypeResource:
		if block.Resource == nil {
			return errors.New("resource field is required for resource content block")
		}
	case api.ContentBlockTypeResourceLink:
		if block.ResourceLink == nil {
			return errors.New("resource_link field is required for resource_link content block")
		}
		if block.ResourceLink.Uri == "" {
			return errors.New("uri is required for resource_link content block")
		}
	}

	return nil
}

// ValidateContentBlocks validates a collection of content blocks.
func ValidateContentBlocks(blocks []api.ContentBlock) error {
	if len(blocks) == 0 {
		return errors.New("content blocks cannot be empty")
	}

	for i, block := range blocks {
		if err := ValidateContentBlock(block); err != nil {
			return fmt.Errorf("invalid content block at index %d: %w", i, err)
		}
	}

	return nil
}

// Content utility functions

// ExtractText extracts all text content from content blocks.
func ExtractText(blocks []api.ContentBlock) string {
	var result string
	for _, block := range blocks {
		if block.Type == api.ContentBlockTypeText && block.Text != nil {
			if result != "" {
				result += "\n"
			}
			result += block.Text.Text
		}
	}
	return result
}

// ExtractTextFromInterface extracts text from []interface{} content blocks.
func ExtractTextFromInterface(content []interface{}) string {
	var result string
	for _, item := range content {
		switch v := item.(type) {
		case string:
			if result != "" {
				result += "\n"
			}
			result += v
		case api.ContentBlock:
			if v.Type == api.ContentBlockTypeText && v.Text != nil {
				if result != "" {
					result += "\n"
				}
				result += v.Text.Text
			}
		case map[string]interface{}:
			// Handle map representation of content block
			if typeVal, typeOk := v["type"].(string); typeOk && typeVal == "text" {
				if textVal, textOk := v["text"].(string); textOk {
					if result != "" {
						result += "\n"
					}
					result += textVal
				}
			}
		}
	}
	return result
}

// FilterContentByType filters content blocks by type.
func FilterContentByType(blocks []api.ContentBlock, contentType api.ContentBlockType) []api.ContentBlock {
	var result []api.ContentBlock
	for _, block := range blocks {
		if block.Type == contentType {
			result = append(result, block)
		}
	}
	return result
}

// GetTextBlocks returns all text content blocks.
func GetTextBlocks(blocks []api.ContentBlock) []api.ContentBlockText {
	var result []api.ContentBlockText
	for _, block := range blocks {
		if block.Type == api.ContentBlockTypeText && block.Text != nil {
			result = append(result, *block.Text)
		}
	}
	return result
}

// GetImageBlocks returns all image content blocks.
func GetImageBlocks(blocks []api.ContentBlock) []api.ContentBlockImage {
	var result []api.ContentBlockImage
	for _, block := range blocks {
		if block.Type == api.ContentBlockTypeImage && block.Image != nil {
			result = append(result, *block.Image)
		}
	}
	return result
}

// GetAudioBlocks returns all audio content blocks.
func GetAudioBlocks(blocks []api.ContentBlock) []api.ContentBlockAudio {
	var result []api.ContentBlockAudio
	for _, block := range blocks {
		if block.Type == api.ContentBlockTypeAudio && block.Audio != nil {
			result = append(result, *block.Audio)
		}
	}
	return result
}

// CountContentTypes counts the number of each content type in the collection.
func CountContentTypes(blocks []api.ContentBlock) map[api.ContentBlockType]int {
	counts := make(map[api.ContentBlockType]int)
	for _, block := range blocks {
		counts[block.Type]++
	}
	return counts
}

// HasContentType checks if any block has the specified type.
func HasContentType(blocks []api.ContentBlock, contentType api.ContentBlockType) bool {
	for _, block := range blocks {
		if block.Type == contentType {
			return true
		}
	}
	return false
}

// ConvertToInterface converts content blocks to []interface{} for API compatibility.
func ConvertToInterface(blocks []api.ContentBlock) []interface{} {
	result := make([]interface{}, len(blocks))
	for i, block := range blocks {
		result[i] = block
	}
	return result
}
