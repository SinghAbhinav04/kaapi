package domain

// Mail is a message to send from the resort's connected mailbox. InReplyTo (the guest's
// Message-ID) and ThreadID make it a reply in the guest's existing conversation.
type Mail struct {
	To        string
	Subject   string
	Body      string
	HTML      string // optional: sent with Body as multipart/alternative
	InReplyTo string
	ThreadID  string
}

// SentMail is what the mail provider returned for a delivered message.
type SentMail struct {
	MessageID string
	ThreadID  string
}
