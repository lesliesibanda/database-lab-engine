import { useParams } from 'react-router-dom'

import { Instance as InstancePage } from '@postgres.ai/shared/pages/Instance'

import { ConsoleBreadcrumbsWrapper } from 'components/ConsoleBreadcrumbs/ConsoleBreadcrumbsWrapper'
import { ROUTES } from 'config/routes'
import { getInstance } from 'api/instances/getInstance'
import { getSnapshots } from 'api/snapshots/getSnapshots'
import { destroyClone } from 'api/clones/destroyClone'
import { resetClone } from 'api/clones/resetClone'
import { bannersStore } from 'stores/banners'
import { getWSToken } from 'api/instances/getWSToken'
import { getConfig } from 'api/configs/getConfig'
import { getFullConfig } from 'api/configs/getFullConfig'
import { testDbSource } from 'api/configs/testDbSource'
import { updateConfig } from 'api/configs/updateConfig'
import { getEngine } from 'api/engine/getEngine'
import { initWS } from 'api/engine/initWS'

type Params = {
  org: string
  project?: string
  instanceId: string
}

export const Instance = () => {
  const params = useParams<Params>()

  const routes = {
    createClone: () =>
      params.project
        ? ROUTES.ORG.PROJECT.INSTANCES.INSTANCE.CLONES.ADD.createPath({
            org: params.org,
            project: params.project,
            instanceId: params.instanceId,
          })
        : ROUTES.ORG.INSTANCES.INSTANCE.CLONES.ADD.createPath(params),

    clone: (cloneId: string) =>
      params.project
        ? ROUTES.ORG.PROJECT.INSTANCES.INSTANCE.CLONES.CLONE.createPath({
            org: params.org,
            project: params.project,
            instanceId: params.instanceId,
            cloneId,
          })
        : ROUTES.ORG.INSTANCES.INSTANCE.CLONES.CLONE.createPath({
            ...params,
            cloneId,
          }),
  }

  const api = {
    getInstance,
    getSnapshots,
    destroyClone,
    resetClone,
    getWSToken,
    getConfig,
    getFullConfig,
    updateConfig,
    testDbSource,
    getEngine,
    initWS,
  }

  const callbacks = {
    showDeprecatedApiBanner: bannersStore.showDeprecatedApi,
    hideDeprecatedApiBanner: bannersStore.hideDeprecatedApi,
  }

  const elements = {
    breadcrumbs: (
      <ConsoleBreadcrumbsWrapper
        hasDivider
        org={params.org}
        project={params.project}
        breadcrumbs={[
          { name: 'Database Lab Instances', url: 'instances' },
          {
            name: `Instance #${params.instanceId} ${
              params.project ? `(${params.project})` : ''
            }`,
            url: null,
          },
        ]}
      />
    ),
  }

  return (
    <InstancePage
      isPlatform
      title={`Database Lab instance #${params.instanceId} ${
        params.project ? `(${params.project})` : ''
      }`}
      instanceId={params.instanceId}
      routes={routes}
      api={api}
      callbacks={callbacks}
      elements={elements}
    />
  )
}
