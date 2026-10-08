import { useEffect, useMemo, useState } from 'react';
import { listFunctions as getFunctionMetadataList } from '../clients/functionsClient';
import { Function, FunctionListItem as FunctionMetadata } from '../types';
import { errorMessage } from '../utils/utils';

export function useFunctions(
  namespace: string,
  connectionId?: number,
  refreshKey?: number,
): {
  functions: Function[];
  loaded: boolean;
  errors?: string[];
} {
  // Source 1: fetch function metadata list from backend
  const [functionMetadataList, setFunctionMetadataList] = useState<FunctionMetadata[]>([]);
  const [listLoaded, setListLoaded] = useState(false);
  const [listError, setListError] = useState<string>();
  const [prevNamespace, setPrevNamespace] = useState(namespace);
  const [prevConnectionId, setPrevConnectionId] = useState(connectionId);

  if (namespace !== prevNamespace || connectionId !== prevConnectionId) {
    setPrevNamespace(namespace);
    setPrevConnectionId(connectionId);
    setFunctionMetadataList([]);
    setListLoaded(false);
    setListError(undefined);
  }

  useEffect(() => {
    let ignore = false;

    (async () => {
      if (connectionId === undefined) {
        setListLoaded(true);
        return;
      }

      try {
        const list = await getFunctionMetadataList(namespace);
        if (!ignore) {
          setFunctionMetadataList(list);
          setListLoaded(true);
          setListError(undefined);
        }
      } catch (err) {
        if (!ignore) {
          setListLoaded(true);
          setListError(errorMessage(err));
        }
      }
    })();

    return () => {
      ignore = true;
    };
  }, [connectionId, namespace, refreshKey]);

  const functions: Function[] = useMemo(
    () =>
      functionMetadataList.map((item) => ({
        ...item,
        status: { cluster: { status: 'NotDeployed' }, workflow: { status: 'None' } },
      })),
    [functionMetadataList],
  );

  const errors: string[] = useMemo(
    () => [listError].filter((errMsg): errMsg is string => !!errMsg),
    [listError],
  );

  return { functions, loaded: listLoaded, errors };
}
