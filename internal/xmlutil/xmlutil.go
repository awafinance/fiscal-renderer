package xmlutil

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

type Node struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Text     string     `xml:",chardata"`
	Children []*Node    `xml:",any"`
}

func Parse(data []byte) (*Node, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if start, ok := token.(xml.StartElement); ok {
			var node Node
			if err := decoder.DecodeElement(&node, &start); err != nil {
				return nil, err
			}
			return &node, nil
		}
	}
}

func ParseString(data string) (*Node, error) {
	return Parse([]byte(data))
}

func ParseReader(r io.Reader) (*Node, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func (n *Node) Find(local string) *Node {
	if n == nil {
		return nil
	}
	if n.XMLName.Local == local {
		return n
	}
	for _, child := range n.Children {
		if found := child.Find(local); found != nil {
			return found
		}
	}
	return nil
}

func (n *Node) FindAll(local string) []*Node {
	var found []*Node
	n.findAll(local, &found)
	return found
}

func (n *Node) findAll(local string, found *[]*Node) {
	if n == nil {
		return
	}
	if n.XMLName.Local == local {
		*found = append(*found, n)
	}
	for _, child := range n.Children {
		child.findAll(local, found)
	}
}

func (n *Node) TextContent() string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Text)
}

func (n *Node) RawTextContent() string {
	if n == nil {
		return ""
	}
	return n.Text
}

func Text(root *Node, local string) string {
	if root == nil {
		return ""
	}
	return root.Find(local).TextContent()
}

func RawText(root *Node, local string) string {
	if root == nil {
		return ""
	}
	return root.Find(local).RawTextContent()
}

func (n *Node) Attr(local string) string {
	if n == nil {
		return ""
	}
	for _, attr := range n.Attrs {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}
