import type { Metadata } from 'next'
import './globals.css'
import { AuthProvider } from '@/contexts/AuthContext'
import { LanguageProvider } from '@/contexts/LanguageContext'
import { ProblemsStateProvider } from '@/contexts/ProblemsStateContext'

export const metadata: Metadata = {
  title: '掉心心',
  description: '选择与结果不一致，掉小心心了！',
  icons: {
    icon: '/favicon.ico',
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body>
        <LanguageProvider>
          <AuthProvider>
            <ProblemsStateProvider>{children}</ProblemsStateProvider>
          </AuthProvider>
        </LanguageProvider>
      </body>
    </html>
  )
}

