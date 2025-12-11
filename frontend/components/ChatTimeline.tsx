'use client';

import ChatMessage, { ChatMessageData } from './ChatMessage';

interface ChatTimelineProps {
  messages: ChatMessageData[];
  userAvatar?: string;
  merchantAvatar?: string;
}

export default function ChatTimeline({ 
  messages,
  userAvatar = '/avatars/avatar_1.png',
  merchantAvatar = '/avatars/avatar_2.png'
}: ChatTimelineProps) {
  return (
    <div className="flex flex-col">
      {messages.map((message, index) => (
        <ChatMessage
          key={index}
          message={message}
          userAvatar={userAvatar}
          merchantAvatar={merchantAvatar}
        />
      ))}
    </div>
  );
}
