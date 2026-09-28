import { createContext, ReactNode, useState } from 'react';
import { ForgeUser, PAT_KEY, USER_KEY } from '../services/types';

interface ForgeConnection {
  isActive: boolean;
  user: ForgeUser;
  connectionId: number;
  connectToForge: (user: ForgeUser) => void;
}

export const ForgeConnectionContext = createContext<ForgeConnection>({
  isActive: false,
  user: { name: '' },
  connectionId: 0,
  connectToForge: () => {},
});

export function ForgeConnectionProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [isActive, setIsActive] = useState<boolean>(() => !!sessionStorage.getItem(PAT_KEY));
  const [user, setUser] = useState<ForgeUser>(readStoredUser);
  const [connectionId, setConnectionId] = useState(0);

  const connectToForge = (forgeUser: ForgeUser) => {
    setUser(forgeUser);
    setIsActive(true);
    setConnectionId((id) => id + 1);
  };

  return (
    <ForgeConnectionContext.Provider value={{ isActive, user, connectionId, connectToForge }}>
      {children}
    </ForgeConnectionContext.Provider>
  );
}

function readStoredUser(): ForgeUser {
  const userJson = sessionStorage.getItem(USER_KEY);
  if (!userJson) return { name: '' };
  try {
    return JSON.parse(userJson) as ForgeUser;
  } catch {
    return { name: '' };
  }
}
