import { createBrowserRouter } from 'react-router-dom'

import Login from './pages/Login'
import SetupWizard from './pages/SetupWizard'
import Dashboard from './pages/Dashboard'
import CallFlows from './pages/CallFlows'
import Trunks from './pages/Trunks'
import InboundNumbers from './pages/InboundNumbers'
import Extensions from './pages/Extensions'
import VoicemailBoxes from './pages/VoicemailBoxes'
import RingGroups from './pages/RingGroups'
import IVRMenus from './pages/IVRMenus'
import TimeSwitches from './pages/TimeSwitches'
import ConferenceBridges from './pages/ConferenceBridges'
import Recordings from './pages/Recordings'
import CallHistory from './pages/CallHistory'
import Settings from './pages/Settings'
import NotFound from './pages/NotFound'

const router = createBrowserRouter([
  // Public routes (no layout)
  { path: '/login', element: <Login /> },
  { path: '/setup', element: <SetupWizard /> },

  // App routes (will get layout wrapper in later sprint)
  { path: '/', element: <Dashboard /> },
  { path: '/call-flows', element: <CallFlows /> },
  { path: '/trunks', element: <Trunks /> },
  { path: '/inbound-numbers', element: <InboundNumbers /> },
  { path: '/extensions', element: <Extensions /> },
  { path: '/voicemail', element: <VoicemailBoxes /> },
  { path: '/ring-groups', element: <RingGroups /> },
  { path: '/ivr-menus', element: <IVRMenus /> },
  { path: '/time-switches', element: <TimeSwitches /> },
  { path: '/conferences', element: <ConferenceBridges /> },
  { path: '/recordings', element: <Recordings /> },
  { path: '/call-history', element: <CallHistory /> },
  { path: '/settings', element: <Settings /> },

  // Catch-all
  { path: '*', element: <NotFound /> },
])

export default router
