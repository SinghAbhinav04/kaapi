export const CODE_LENGTH = 6;

export const COPY = {
  title: 'Staff sign-in',
  leadPassword: 'Reservations, sales and management.',
  leadCode: 'Enter the code we just emailed you.',
  email: 'Work email',
  password: 'Password',
  codeLabel: (sentTo: string) => `Code sent to ${sentTo}`,
  codeHint: 'Check your work inbox. The code works for 5 minutes.',
  wrongPassword: 'That email and password don’t match.',
  continue: 'Continue',
  verify: 'Verify and sign in',
  otherAccount: 'Use a different account',
  demo: 'Demo accounts are listed in deployment.md.',
} as const;
