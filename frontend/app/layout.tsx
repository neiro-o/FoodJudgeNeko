import type { Metadata } from 'next'
import './globals.css'
import { AuthProvider } from '@/contexts/AuthContext'
import { LanguageProvider } from '@/contexts/LanguageContext'
import { ProblemsStateProvider } from '@/contexts/ProblemsStateContext'
import { ThemeProvider } from '@/contexts/ThemeContext'

export const metadata: Metadata = {
  title: '掉心心',
  description: '选择与结果不一致，掉小心心了！',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <ThemeProvider>
          <LanguageProvider>
            <AuthProvider>
              <ProblemsStateProvider>{children}</ProblemsStateProvider>
            </AuthProvider>
          </LanguageProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}

