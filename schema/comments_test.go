package schema

import (
	"reflect"
	"testing"
)

func TestExtractCommentFromTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{
			name: "simple doc tag",
			tag:  `json:"name" doc:"User's name"`,
			want: "User's name",
		},
		{
			name: "doc tag with special characters",
			tag:  `json:"email" doc:"User's email address (required)" validate:"email"`,
			want: "User's email address (required)",
		},
		{
			name: "no doc tag",
			tag:  `json:"id" validate:"required"`,
			want: "",
		},
		{
			name: "empty doc tag",
			tag:  `json:"field" doc:""`,
			want: "",
		},
		{
			name: "doc tag with quotes",
			tag:  `json:"desc" doc:"This is a \"quoted\" value"`,
			want: `This is a \"quoted\" value`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCommentFromTag(tt.tag)
			if got != tt.want {
				t.Errorf("ExtractCommentFromTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractProtoDoc(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{
			name: "simple protoDoc",
			tag:  `protoDoc:"This is a message description"`,
			want: "This is a message description",
		},
		{
			name: "protoDoc with newlines",
			tag:  `protoDoc:"Line 1\nLine 2"`,
			want: `Line 1\nLine 2`,
		},
		{
			name: "no protoDoc",
			tag:  `json:"field"`,
			want: "",
		},
		{
			name: "empty protoDoc",
			tag:  `protoDoc:""`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractProtoDoc(tt.tag)
			if got != tt.want {
				t.Errorf("ExtractProtoDoc() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathBuilder(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		pb := NewPathBuilder()

		// Test Push
		pb.Push(4).Push(0).Push(2).Push(1)
		path := pb.Build()
		expected := []int32{4, 0, 2, 1}
		if !reflect.DeepEqual(path, expected) {
			t.Errorf("Build() = %v, want %v", path, expected)
		}

		// Test Pop
		pb.Pop()
		path = pb.Build()
		expected = []int32{4, 0, 2}
		if !reflect.DeepEqual(path, expected) {
			t.Errorf("After Pop(), Build() = %v, want %v", path, expected)
		}

		// Test Reset
		pb.Reset()
		path = pb.Build()
		if len(path) != 0 {
			t.Errorf("After Reset(), Build() = %v, want empty", path)
		}
	})

	t.Run("clone", func(t *testing.T) {
		pb1 := NewPathBuilder()
		pb1.Push(1).Push(2).Push(3)

		pb2 := pb1.Clone()
		pb2.Push(4)

		// Original should not be affected
		path1 := pb1.Build()
		path2 := pb2.Build()

		if len(path1) != 3 {
			t.Errorf("Original path length = %d, want 3", len(path1))
		}
		if len(path2) != 4 {
			t.Errorf("Cloned path length = %d, want 4", len(path2))
		}
	})
}

func TestSourceCodeInfoBuilder(t *testing.T) {
	t.Run("add location with leading comment", func(t *testing.T) {
		builder := NewSourceCodeInfoBuilder()
		path := []int32{4, 0, 2, 1}
		comment := &CommentInfo{
			Leading: "This is a field comment",
		}

		builder.AddLocation(path, comment)
		sci := builder.Build()

		if sci == nil {
			t.Fatal("Build() returned nil")
		}
		if len(sci.Location) != 1 {
			t.Fatalf("Expected 1 location, got %d", len(sci.Location))
		}

		loc := sci.Location[0]
		if !reflect.DeepEqual(loc.Path, path) {
			t.Errorf("Location path = %v, want %v", loc.Path, path)
		}
		if loc.LeadingComments == nil || *loc.LeadingComments != "This is a field comment" {
			t.Errorf("Leading comment = %v, want 'This is a field comment'", loc.LeadingComments)
		}
	})

	t.Run("add location with all comment types", func(t *testing.T) {
		builder := NewSourceCodeInfoBuilder()
		path := []int32{6, 0}
		comment := &CommentInfo{
			Leading:  "Service documentation",
			Trailing: "End of service",
			Detached: []string{"Detached paragraph 1", "Detached paragraph 2"},
		}

		builder.AddLocation(path, comment)
		sci := builder.Build()

		loc := sci.Location[0]
		if loc.LeadingComments == nil || *loc.LeadingComments != "Service documentation" {
			t.Errorf("Leading comment incorrect")
		}
		if loc.TrailingComments == nil || *loc.TrailingComments != "End of service" {
			t.Errorf("Trailing comment incorrect")
		}
		if len(loc.LeadingDetachedComments) != 2 {
			t.Errorf("Expected 2 detached comments, got %d", len(loc.LeadingDetachedComments))
		}
	})

	t.Run("no location for empty comment", func(t *testing.T) {
		builder := NewSourceCodeInfoBuilder()
		path := []int32{4, 0}
		comment := &CommentInfo{} // Empty comment

		builder.AddLocation(path, comment)
		sci := builder.Build()

		if sci != nil {
			t.Error("Expected nil SourceCodeInfo for empty comments")
		}
	})
}

func TestFormatComment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple comment",
			input: "This is a comment",
			want:  "This is a comment",
		},
		{
			name:  "comment with leading/trailing space",
			input: "  This is a comment  ",
			want:  "This is a comment",
		},
		{
			name:  "multi-line comment",
			input: "Line 1\nLine 2\nLine 3",
			want:  "Line 1\n Line 2\n Line 3",
		},
		{
			name:  "multi-line with empty lines",
			input: "Line 1\n\nLine 3",
			want:  "Line 1\n\n Line 3",
		},
		{
			name:  "empty comment",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatComment(tt.input)
			if got != tt.want {
				t.Errorf("formatComment() = %q, want %q", got, tt.want)
			}
		})
	}
}
